//go:build windows

// INPUT: 产品持久Host接口、固定helper配置与完整命令。
// OUTPUT: Reserve→prepared持久化→一次ClaimStart→cleaned且真实退出的监督生命周期。
// POS: unknown保留Process owner，断链或退出单独不能确认资源清理。
package windowssandbox

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Process struct {
	mu               sync.Mutex
	closeMu          sync.Mutex
	native           *nativeMachine
	config           Config
	intent           Intent
	prepared         controlFrame
	outcome          Outcome
	result           error
	done             chan struct{}
	updates          chan struct{}
	generation       uint64
	cleanupError     error
	finishPending    *Outcome
	finishProofError error
	locallyClosed    bool
	phase            string
	cleanupAttempted bool
	stop             func() bool
}

func (p *Process) Intent() Intent { return p.intent }
func (p *Process) Streams() Streams {
	return Streams{Stdin: p.native.pipes[1], Stdout: p.native.pipes[2], Stderr: p.native.pipes[3]}
}

// Start Reserve先于CreateProcess；返回非nil Process无论是否成功都必须保留到Close确认。
func Start(ctx context.Context, config Config, command Command) (p *Process, result error) {
	lifetime, cancel := context.WithCancel(context.Background())
	p = &Process{native: &nativeMachine{ctx: lifetime, cancel: cancel}, config: config, done: make(chan struct{}), updates: make(chan struct{}), phase: "prepared"}
	reserved := false
	monitoring := false
	defer func() {
		if result == nil {
			return
		}
		p.result = result
		p.outcome = Outcome{Prepared: Prepared{LaunchID: p.intent.LaunchID}, Reason: result.Error()}
		if p.prepared.Kind == "prepared" {
			p.outcome.Prepared = Prepared{LaunchID: p.intent.LaunchID, ExecutionID: p.prepared.ExecutionID, PrepareDigest: p.prepared.PrepareDigest, ManifestDigest: p.prepared.ManifestDigest}
		}
		if reserved {
			finish, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			p.result = errors.Join(p.result, config.Host.Finish(finish, p.intent, p.outcome))
			cancel()
		}
		result = p.result
		if p.prepared.Kind == "prepared" && !monitoring {
			go p.observe()
			cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, cleanupErr := p.requestCleanup(cleanup)
			result = errors.Join(result, cleanupErr)
			cancel()
			return
		}
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		closeErr := p.native.close(cleanup)
		if closeErr == nil {
			p.locallyClosed = true
		}
		p.result = errors.Join(p.result, closeErr)
		cancel()
		result = p.result
		if !monitoring {
			p.phase = "terminal"
			close(p.done)
		}
	}()
	if ctx == nil || config.Host == nil {
		return p, errors.New("machine sandbox requires Host and context")
	}
	if err := ctx.Err(); err != nil {
		return p, err
	}
	command.Arguments = append([]string(nil), command.Arguments...)
	command.Environment = append([]string(nil), command.Environment...)
	body, err := json.Marshal(command)
	if err != nil || len(body) > maxControlFrame {
		return p, errors.New("machine command exceeds bound")
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return p, err
	}
	p.intent = Intent{Version: 1, LaunchID: hex.EncodeToString(nonce[:]), HelperSHA256: config.HelperSHA256, CommandDigest: sha256.Sum256(body)}
	if err = p.native.prepare(config); err != nil {
		return p, err
	}
	reserved = true
	options, err := config.Host.Reserve(ctx, p.intent, command)
	if err != nil {
		return p, err
	}
	if len(options) == 0 || len(options) > maxControlFrame || !json.Valid(options) {
		return p, errors.New("Host returned invalid preparation options")
	}
	if err = p.native.launch(ctx); err != nil {
		return p, err
	}
	prepare, err := json.Marshal(controlFrame{Version: 1, Kind: "prepare", LaunchID: p.intent.LaunchID, Options: options})
	if err != nil {
		return p, err
	}
	if err = p.native.pipes[0].writeFrame(ctx, prepare); err != nil {
		return p, err
	}
	reply, err := p.native.pipes[0].readFrame(ctx)
	if err != nil {
		return p, err
	}
	p.prepared, err = decodeControlReply(reply, p.intent.LaunchID)
	if err != nil {
		return p, err
	}
	if p.prepared.Kind != "prepared" {
		return p, errors.New("machine helper did not confirm preparation")
	}
	actual := Prepared{LaunchID: p.intent.LaunchID, ExecutionID: p.prepared.ExecutionID, PrepareDigest: p.prepared.PrepareDigest, ManifestDigest: p.prepared.ManifestDigest}
	if err = config.Host.RecordPrepared(ctx, p.intent, actual); err != nil {
		return p, err
	}
	if err = config.Host.ClaimStart(ctx, p.intent, actual); err != nil {
		return p, err
	}
	start, err := continuationFrame("start", p.prepared)
	if err != nil {
		return p, err
	}
	if err = p.native.pipes[0].writeFrame(ctx, start); err != nil {
		return p, err
	}
	monitoring = true
	p.phase = "running"
	go p.observe()
	p.stop = context.AfterFunc(ctx, func() {
		bounded, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = p.requestCleanup(bounded)
	})
	return p, nil
}

// observe 收到绑定cleaned后还须等待真实helper退出，Finish失败不报告Confirmed。
func (p *Process) observe() {
	for {
		outcome := Outcome{Prepared: Prepared{LaunchID: p.intent.LaunchID, ExecutionID: p.prepared.ExecutionID, PrepareDigest: p.prepared.PrepareDigest, ManifestDigest: p.prepared.ManifestDigest}}
		unknownReceipt := false
		body, err := p.native.pipes[0].readFrame(p.native.ctx)
		if err == nil {
			var receipt controlFrame
			receipt, err = decodeControlReply(body, p.intent.LaunchID)
			if err == nil {
				if !receipt.matchesPrepared(p.prepared) {
					err = errors.New("helper cleanup receipt is unknown or differs from prepared execution")
				} else if receipt.Kind == "unknown" {
					unknownReceipt = true
					err = errors.New(receipt.Reason)
				} else if receipt.Kind != "cleaned" {
					err = errors.New("unexpected helper cleanup phase")
				} else {
					var helperCode uint32
					helperCode, err = p.native.wait(p.native.ctx)
					if err == nil {
						err = p.native.verifyJobEmpty()
					}
					if err == nil {
						outcome.Cleaned = true
						outcome.ExitCode = receipt.ExitCode
						if helperCode != 0 {
							err = fmt.Errorf("machine helper failed after cleanup with exit code %d", helperCode)
						}
					}
				}
			}
		}
		if err != nil {
			outcome.Reason = err.Error()
		}
		finish, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		proof := outcome
		proofError := err
		finishErr := p.config.Host.Finish(finish, p.intent, outcome)
		err = errors.Join(err, finishErr)
		if finishErr != nil {
			outcome.Cleaned = false
			outcome.Reason = err.Error()
		}
		cancel()
		p.mu.Lock()
		if finishErr != nil && proof.Cleaned {
			p.finishPending = &proof
			p.finishProofError = proofError
		}
		p.outcome = outcome
		p.result = err
		if unknownReceipt {
			p.phase = "unknown"
			p.cleanupAttempted = false
		} else {
			p.phase = "terminal"
			close(p.done)
		}
		p.generation++
		close(p.updates)
		p.updates = make(chan struct{})
		p.mu.Unlock()
		if !unknownReceipt {
			return
		}
	}
}
func (p *Process) requestCleanup(ctx context.Context) (uint64, error) {
	p.mu.Lock()
	generation := p.generation
	if p.cleanupError != nil {
		err := p.cleanupError
		p.mu.Unlock()
		return generation, err
	}
	if p.cleanupAttempted {
		p.mu.Unlock()
		return generation, nil
	}
	select {
	case <-p.done:
		p.mu.Unlock()
		return generation, nil
	default:
	}
	p.cleanupAttempted = true
	kind := "cleanup"
	if p.phase == "running" {
		kind = "cancel"
	}
	prepared := p.prepared
	p.mu.Unlock()
	body, err := continuationFrame(kind, prepared)
	if err == nil {
		err = p.native.pipes[0].writeFrame(ctx, body)
	}
	if err != nil {
		p.mu.Lock()
		p.cleanupError = err
		p.mu.Unlock()
		p.native.cancel()
	}
	return generation, err
}
func (p *Process) Wait(ctx context.Context) (Outcome, error) {
	return p.waitAfter(ctx, 0)
}

// waitAfter 广播代次不会被另一个Wait消费；Close只等其清理请求之后的新结果。
func (p *Process) waitAfter(ctx context.Context, generation uint64) (Outcome, error) {
	for {
		p.mu.Lock()
		if p.phase == "terminal" || p.generation > generation {
			outcome, err := p.outcome, p.result
			p.mu.Unlock()
			return outcome, err
		}
		changed := p.updates
		p.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			return Outcome{Prepared: Prepared{LaunchID: p.intent.LaunchID}, Reason: ctx.Err().Error()}, ctx.Err()
		}
	}
}

// Close 不强杀运行中helper；未知/持久确认失败均保留owner并返回错误。
func (p *Process) Close(ctx context.Context) error {
	p.closeMu.Lock()
	defer p.closeMu.Unlock()
	if p.stop != nil {
		p.stop()
	}
	generation, err := p.requestCleanup(ctx)
	if err != nil {
		return err
	}
	outcome, err := p.waitAfter(ctx, generation)
	if !outcome.Cleaned {
		outcome, err = p.retryFinish(ctx, outcome, err)
	}
	if !outcome.Cleaned {
		return errors.Join(errors.New("machine cleanup remains unconfirmed"), err)
	}
	closeErr := p.native.close(ctx)
	if closeErr == nil {
		p.mu.Lock()
		p.locallyClosed = true
		p.mu.Unlock()
	}
	return errors.Join(err, closeErr)
}

// CleanupPending 只报告原owner是否仍持实际资源，不用断链/主进程退出推断完成。
func (p *Process) CleanupPending() bool { p.mu.Lock(); defer p.mu.Unlock(); return !p.locallyClosed }

// retryFinish 仅重交同一已证实cleaned的持久结果，不重发start或任何设备操作。
func (p *Process) retryFinish(ctx context.Context, outcome Outcome, prior error) (Outcome, error) {
	p.mu.Lock()
	pending := p.finishPending
	proofError := p.finishProofError
	p.mu.Unlock()
	if pending == nil {
		return outcome, prior
	}
	if err := p.config.Host.Finish(ctx, p.intent, *pending); err != nil {
		return outcome, errors.Join(prior, err)
	}
	p.mu.Lock()
	p.outcome = *pending
	p.result = proofError
	p.finishPending = nil
	p.generation++
	close(p.updates)
	p.updates = make(chan struct{})
	p.mu.Unlock()
	return *pending, proofError
}
