/* evline: см. include/s2p/evline.h и docs/steer-api.md, раздел 2. */
#include "s2p/evline.h"

#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

/* Хвост строки: закрывающая } и \n — место под них оставлено всегда. */
#define TAIL 2

static void put(struct s2p_ev *e, const char *s, size_t len) {
    if (e->overflow) return;
    if (e->n + len + TAIL > sizeof e->buf) { e->overflow = 1; return; }
    memcpy(e->buf + e->n, s, len);
    e->n += len;
}

static void put_str(struct s2p_ev *e, const char *s) {
    static const char hex[] = "0123456789abcdef";
    put(e, "\"", 1);
    for (const unsigned char *p = (const unsigned char *)s; *p; p++) {
        char esc[6];
        switch (*p) {
        case '"':  put(e, "\\\"", 2); break;
        case '\\': put(e, "\\\\", 2); break;
        case '\n': put(e, "\\n", 2); break;
        case '\r': put(e, "\\r", 2); break;
        case '\t': put(e, "\\t", 2); break;
        default:
            if (*p < 0x20) {
                esc[0] = '\\'; esc[1] = 'u'; esc[2] = '0'; esc[3] = '0';
                esc[4] = hex[*p >> 4]; esc[5] = hex[*p & 15];
                put(e, esc, 6);
            } else {
                put(e, (const char *)p, 1);
            }
        }
    }
    put(e, "\"", 1);
}

void s2p_ev_begin(struct s2p_ev *e, const char *ev) {
    e->n = 0;
    e->overflow = 0;
    put(e, "{\"ev\":", 6);
    put_str(e, ev);
}

void s2p_ev_str(struct s2p_ev *e, const char *key, const char *val) {
    put(e, ",", 1);
    put_str(e, key);
    put(e, ":", 1);
    put_str(e, val ? val : "");
}

void s2p_ev_int(struct s2p_ev *e, const char *key, long long val) {
    char num[32];
    int k = snprintf(num, sizeof num, "%lld", val);
    put(e, ",", 1);
    put_str(e, key);
    put(e, ":", 1);
    put(e, num, (size_t)k);
}

int s2p_ev_send(struct s2p_ev *e, int fd) {
    if (e->overflow) return -1;
    if (fd < 0) {
        const char *s = getenv("STEER_EVENT_FD");
        fd = s && *s ? atoi(s) : 3;
    }
    e->buf[e->n++] = '}';
    e->buf[e->n++] = '\n';
    ssize_t w;
    do w = write(fd, e->buf, e->n); while (w < 0 && errno == EINTR);
    e->n -= TAIL;   /* событие можно дополнить и отправить снова */
    if (w == (ssize_t)(e->n + TAIL)) return 0;
    if (w < 0 && (errno == EAGAIN || errno == EWOULDBLOCK)) return 1;
    return -1;
}

int s2p_ev_hello(const char *ver, const char *mod) {
    struct s2p_ev e;
    s2p_ev_begin(&e, "hello");
    s2p_ev_str(&e, "api", S2P_STEER_API);
    s2p_ev_str(&e, "ver", ver);
    s2p_ev_str(&e, "mod", mod);
    return s2p_ev_send(&e, -1);
}

int s2p_ev_up(int watch, const char *dev) {
    struct s2p_ev e;
    s2p_ev_begin(&e, "up");
    s2p_ev_int(&e, "watch", watch ? 1 : 0);
    if (dev) s2p_ev_str(&e, "dev", dev);
    return s2p_ev_send(&e, -1);
}

int s2p_ev_down(const char *why) {
    struct s2p_ev e;
    s2p_ev_begin(&e, "down");
    s2p_ev_str(&e, "why", why);
    return s2p_ev_send(&e, -1);
}

int s2p_ev_ready(void) {
    struct s2p_ev e;
    s2p_ev_begin(&e, "ready");
    return s2p_ev_send(&e, -1);
}
