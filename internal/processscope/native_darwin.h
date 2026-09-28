// INPUT: 固定内核 coalition 与 audit identity。
// OUTPUT: errno 风格状态；只提供观察与精确信号。
// POS: cgo 内部 ABI，不包含 job 创建、权限获取或裸 PID 终止。
#include <stdint.h>

struct nx_scope_process { uint64_t coalition; uint32_t audit[8]; };
int nx_scope_available(void);
int nx_scope_inspect(int pid, struct nx_scope_process *out);
int nx_scope_exists(uint64_t coalition);
int nx_scope_members(uint64_t coalition, struct nx_scope_process **out, int *count);
int nx_scope_signal(struct nx_scope_process *target, int signal);
