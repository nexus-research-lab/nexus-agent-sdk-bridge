//go:build darwin && cgo

// INPUT: 已登记集合和原生进程观察。
// OUTPUT: 两次核对的 audit identity、集合存在性及精确信号结果。
// POS: 实验组件的 macOS 原生边界；不以空列表或任务计数确认退出。
#include "native_darwin.h"
#include <dlfcn.h>
#include <errno.h>
#include <libproc.h>
#include <mach/mach.h>
#include <stddef.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <sys/un.h>

// XNU proc_info_private.h；结构不符必须失败，不读取未验证的偏移。
struct nx_coalition_ids { uint64_t ids[2]; uint64_t reserved[3]; };
typedef int (*nx_usage_fn)(uint64_t, void *, size_t);
typedef int (*nx_signal_fn)(audit_token_t *, int);

int nx_scope_available(void) {
    return dlsym(RTLD_DEFAULT,"coalition_info_resource_usage") &&
        dlsym(RTLD_DEFAULT,"proc_signal_with_audittoken") ? 0 : ENOSYS;
}

static int nx_read_audit(int pid, audit_token_t *token) {
    mach_port_t port=MACH_PORT_NULL;
    kern_return_t result=task_name_for_pid(mach_task_self(),pid,&port);
    if (result==KERN_SUCCESS) {
        mach_msg_type_number_t count=TASK_AUDIT_TOKEN_COUNT;
        result=task_info(port,TASK_AUDIT_TOKEN,(task_info_t)token,&count);
        if (count!=TASK_AUDIT_TOKEN_COUNT) result=KERN_FAILURE;
    }
    if (port!=MACH_PORT_NULL) mach_port_deallocate(mach_task_self(),port);
    // Mach 查询失败不能冒充 ESRCH；可能是权限拒绝或观察缺口。
    return result==KERN_SUCCESS ? 0 : EACCES;
}

static int nx_read_coalition(int pid, uint64_t *out) {
    struct nx_coalition_ids ids={0};
    errno=0;
    int size=proc_pidinfo(pid,20,0,&ids,sizeof(ids));
    if (size!=sizeof(ids)) return size==0 && errno ? errno : EPROTO;
    if (!ids.ids[0]) return EPROTO;
    *out=ids.ids[0];return 0;
}

int nx_scope_inspect(int pid, struct nx_scope_process *out) {
    audit_token_t before={0},after={0};
    int err=nx_read_audit(pid,&before);if(err)return err;
    err=nx_read_coalition(pid,&out->coalition);if(err)return err;
    err=nx_read_audit(pid,&after);if(err)return err;
    if (memcmp(&before,&after,sizeof(before))!=0) return EAGAIN;
    if (before.val[5]!=(uint32_t)pid || before.val[7]==0) return EPROTO;
    memcpy(out->audit,before.val,sizeof(out->audit));return 0;
}

int nx_scope_exists(uint64_t coalition) {
    nx_usage_fn usage=(nx_usage_fn)dlsym(RTLD_DEFAULT,"coalition_info_resource_usage");
    if (!usage)return ENOSYS;
    uint64_t diagnostic[2]={0};errno=0;
    int result=usage(coalition,diagnostic,sizeof(diagnostic));
    // 即使两个计数相等也仍视为存在；只接受内核明确的 ESRCH。
    return result==0 ? 0 : (errno ? errno : EIO);
}

int nx_scope_members(uint64_t coalition, struct nx_scope_process **out, int *count) {
    *out=NULL;*count=0;
    int observed=proc_listallpids(NULL,0);
    if (observed<=0)return errno ? errno : EIO;
    if (observed>1048448)return EOVERFLOW;
    int capacity=observed+128;
    int *pids=calloc((size_t)capacity,sizeof(int));
    struct nx_scope_process *members=calloc((size_t)capacity,sizeof(*members));
    if (!pids || !members){free(pids);free(members);return ENOMEM;}
    int length=proc_listallpids(pids,capacity*(int)sizeof(int));
    int err=0;
    if(length<=0){err=errno ? errno : EIO;goto done;}
    if(length>=capacity){err=ENOBUFS;goto done;}
    for(int i=0;i<length;i++) {
        uint64_t first=0;
        if(pids[i]<=1 || nx_read_coalition(pids[i],&first)!=0 || first!=coalition)continue;
        struct nx_scope_process item={0};
        err=nx_scope_inspect(pids[i],&item);
        if(err==ESRCH || err==EAGAIN){err=0;continue;}
        if(err)goto done;
        if(item.coalition!=coalition)continue;
        members[(*count)++]=item;
    }
done:
    free(pids);
    if(err){free(members);*count=0;return err;}
    *out=members;return 0;
}

int nx_scope_signal(struct nx_scope_process *target, int signal) {
    nx_signal_fn send=(nx_signal_fn)dlsym(RTLD_DEFAULT,"proc_signal_with_audittoken");
    if(!send)return ENOSYS;
    audit_token_t token={0};memcpy(token.val,target->audit,sizeof(target->audit));
    errno=0;int result=send(&token,signal);
    // libproc 的此接口返回正 errno，不能只检查 errno 或 -1。
    return result>=0 ? result : (errno ? errno : EIO);
}

// LOCAL_PEERTOKEN 来源于连接的内核凭据；不接受应用正文中的 PID/token。
int nx_scope_peer(int fd, uint32_t audit[8]) {
    audit_token_t token={0};
    socklen_t size=sizeof(token);
    if (getsockopt(fd,SOL_LOCAL,LOCAL_PEERTOKEN,&token,&size)!=0)
        return errno ? errno : EIO;
    if (size!=sizeof(token) || token.val[5]<=1 || token.val[5]>INT32_MAX || token.val[7]==0)
        return EPROTO;
    memcpy(audit,token.val,sizeof(token.val));
    return 0;
}
