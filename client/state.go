package client

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/runtimeinfo"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type sessionLifecycle struct {
	connectedMu sync.RWMutex
	connected   bool

	closeOnce        sync.Once
	inputCloseOnce   sync.Once
	firstResultOnce  sync.Once
	sessionReadyOnce sync.Once

	stateMu            sync.RWMutex
	sessionID          string
	readErr            error
	lastErrorResult    string
	initializeResponse runtimeinfo.InitializeResponse

	inputStreamMu     sync.RWMutex
	inputStreamActive bool
}

func newSessionLifecycle() *sessionLifecycle {
	return &sessionLifecycle{}
}

func (l *sessionLifecycle) lockConnection() {
	l.connectedMu.Lock()
}

func (l *sessionLifecycle) unlockConnection() {
	l.connectedMu.Unlock()
}

func (l *sessionLifecycle) connectedLocked() bool {
	return l.connected
}

func (l *sessionLifecycle) setConnectedLocked(connected bool) {
	l.connected = connected
}

func (l *sessionLifecycle) isConnected() bool {
	l.connectedMu.RLock()
	defer l.connectedMu.RUnlock()
	return l.connected
}

func (l *sessionLifecycle) setConnected(connected bool) {
	l.connectedMu.Lock()
	l.connected = connected
	l.connectedMu.Unlock()
}

func (l *sessionLifecycle) closeOnceDo(fn func()) {
	l.closeOnce.Do(fn)
}

func (l *sessionLifecycle) inputCloseOnceDo(fn func()) {
	l.inputCloseOnce.Do(fn)
}

func (l *sessionLifecycle) firstResultOnceDo(fn func()) {
	l.firstResultOnce.Do(fn)
}

func (l *sessionLifecycle) sessionReadyOnceDo(fn func()) {
	l.sessionReadyOnce.Do(fn)
}

func (l *sessionLifecycle) resetRuntimeState(sessionID string) {
	l.closeOnce = sync.Once{}
	l.inputCloseOnce = sync.Once{}
	l.firstResultOnce = sync.Once{}
	l.sessionReadyOnce = sync.Once{}
	l.setInputStreamActive(false)
	l.stateMu.Lock()
	l.sessionID = sessionID
	l.readErr = nil
	l.lastErrorResult = ""
	l.initializeResponse = runtimeinfo.InitializeResponse{}
	l.stateMu.Unlock()
}

func (l *sessionLifecycle) setReadError(err error) {
	l.stateMu.Lock()
	defer l.stateMu.Unlock()
	if l.readErr == nil {
		l.readErr = err
	}
}

func (l *sessionLifecycle) readError() error {
	l.stateMu.RLock()
	defer l.stateMu.RUnlock()
	return l.readErr
}

func (l *sessionLifecycle) setLastErrorResult(text string) {
	l.stateMu.Lock()
	l.lastErrorResult = text
	l.stateMu.Unlock()
}

func (l *sessionLifecycle) lastErrorResultValue() string {
	l.stateMu.RLock()
	defer l.stateMu.RUnlock()
	return l.lastErrorResult
}

func (l *sessionLifecycle) setSessionID(sessionID string) {
	l.stateMu.Lock()
	l.sessionID = sessionID
	l.stateMu.Unlock()
}

func (l *sessionLifecycle) sessionIDValue() string {
	l.stateMu.RLock()
	defer l.stateMu.RUnlock()
	return l.sessionID
}

func (l *sessionLifecycle) currentSessionID(defaultSessionID string, optionSessionID string, resumeSessionID string) string {
	l.stateMu.RLock()
	sessionID := l.sessionID
	l.stateMu.RUnlock()
	if sessionID != "" {
		return sessionID
	}
	if optionSessionID != "" {
		return optionSessionID
	}
	if resumeSessionID != "" {
		return resumeSessionID
	}
	return defaultSessionID
}

func (l *sessionLifecycle) setInitializeResponse(response runtimeinfo.InitializeResponse) {
	l.stateMu.Lock()
	l.initializeResponse = response
	l.stateMu.Unlock()
}

func (l *sessionLifecycle) initializeResponseValue() runtimeinfo.InitializeResponse {
	l.stateMu.RLock()
	defer l.stateMu.RUnlock()
	return l.initializeResponse
}

func (l *sessionLifecycle) setInputStreamActive(active bool) {
	l.inputStreamMu.Lock()
	l.inputStreamActive = active
	l.inputStreamMu.Unlock()
}

func (l *sessionLifecycle) inputStreamActiveValue() bool {
	l.inputStreamMu.RLock()
	defer l.inputStreamMu.RUnlock()
	return l.inputStreamActive
}

type sessionStreams struct {
	buffer int

	messages            chan protocol.ReceivedMessage
	readDone            chan struct{}
	firstResult         chan struct{}
	initialSessionReady chan struct{}
	inputClosed         chan struct{}
}

func newSessionStreams(buffer int) *sessionStreams {
	streams := &sessionStreams{buffer: buffer}
	streams.reset()
	return streams
}

func (s *sessionStreams) reset() {
	if s.buffer <= 0 {
		s.buffer = 64
	}
	s.messages = make(chan protocol.ReceivedMessage, s.buffer)
	s.readDone = make(chan struct{})
	s.firstResult = make(chan struct{})
	s.initialSessionReady = make(chan struct{})
	s.inputClosed = make(chan struct{})
}

type controlWaitResult struct {
	Response map[string]any
	Err      error
}

type pendingControlRequests struct {
	counter atomic.Uint64
	mu      sync.Mutex
	waiters map[string]chan controlWaitResult
}

func newPendingControlRequests() *pendingControlRequests {
	return &pendingControlRequests{
		waiters: map[string]chan controlWaitResult{},
	}
}

func (p *pendingControlRequests) nextID() string {
	return fmt.Sprintf("req_%d", p.counter.Add(1))
}

func (p *pendingControlRequests) register(requestID string) <-chan controlWaitResult {
	waiter := make(chan controlWaitResult, 1)
	p.mu.Lock()
	p.waiters[requestID] = waiter
	p.mu.Unlock()
	return waiter
}

func (p *pendingControlRequests) resolve(requestID string, result controlWaitResult) bool {
	p.mu.Lock()
	waiter, ok := p.waiters[requestID]
	if ok {
		delete(p.waiters, requestID)
	}
	p.mu.Unlock()
	if !ok {
		return false
	}
	waiter <- result
	return true
}

func (p *pendingControlRequests) delete(requestID string) bool {
	p.mu.Lock()
	_, ok := p.waiters[requestID]
	if ok {
		delete(p.waiters, requestID)
	}
	p.mu.Unlock()
	return ok
}

func (p *pendingControlRequests) rejectAll(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for requestID, waiter := range p.waiters {
		waiter <- controlWaitResult{Err: err}
		delete(p.waiters, requestID)
	}
}

func (p *pendingControlRequests) reset() {
	p.mu.Lock()
	p.waiters = map[string]chan controlWaitResult{}
	p.mu.Unlock()
}

type inflightControlRequests struct {
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

func newInflightControlRequests() *inflightControlRequests {
	return &inflightControlRequests{
		cancels: map[string]context.CancelFunc{},
	}
}

func (i *inflightControlRequests) add(requestID string, cancel context.CancelFunc) {
	i.mu.Lock()
	i.cancels[requestID] = cancel
	i.mu.Unlock()
}

func (i *inflightControlRequests) remove(requestID string) {
	i.mu.Lock()
	delete(i.cancels, requestID)
	i.mu.Unlock()
}

func (i *inflightControlRequests) cancel(requestID string) bool {
	i.mu.Lock()
	cancel, ok := i.cancels[requestID]
	if ok {
		delete(i.cancels, requestID)
	}
	i.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

func (i *inflightControlRequests) reset() {
	i.mu.Lock()
	i.cancels = map[string]context.CancelFunc{}
	i.mu.Unlock()
}

type registry[T any] struct {
	mu     sync.RWMutex
	values map[string]T
}

func newRegistry[T any]() *registry[T] {
	return &registry[T]{
		values: map[string]T{},
	}
}

func (r *registry[T]) replace(values map[string]T) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.values = make(map[string]T, len(values))
	for name, value := range values {
		r.values[name] = value
	}
}

func (r *registry[T]) snapshot() map[string]T {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]T, len(r.values))
	for name, value := range r.values {
		result[name] = value
	}
	return result
}

func (r *registry[T]) get(name string) (T, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	value, ok := r.values[name]
	return value, ok
}

func (r *registry[T]) reset() {
	r.replace(nil)
}

type hookCallbackRegistry struct {
	counter atomic.Uint64
	mu      sync.RWMutex
	values  map[string]hook.Callback
}

func newHookCallbackRegistry() *hookCallbackRegistry {
	return &hookCallbackRegistry{
		values: map[string]hook.Callback{},
	}
}

func (h *hookCallbackRegistry) register(callback hook.Callback) string {
	if callback == nil {
		return ""
	}
	callbackID := fmt.Sprintf("hook_%d", h.counter.Add(1))
	h.mu.Lock()
	h.values[callbackID] = callback
	h.mu.Unlock()
	return callbackID
}

func (h *hookCallbackRegistry) get(callbackID string) (hook.Callback, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	callback, ok := h.values[callbackID]
	return callback, ok
}

func (h *hookCallbackRegistry) reset() {
	h.mu.Lock()
	h.values = map[string]hook.Callback{}
	h.mu.Unlock()
}
