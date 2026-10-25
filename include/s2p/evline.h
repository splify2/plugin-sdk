/* evline: события помощника и службы ядру steer (docs/steer-api.md, раздел 2).
 *
 * Одна строка JSON на событие, '\n' в конце, один write(2) в STEER_EVENT_FD (по умолчанию 3).
 * Строка не длиннее PIPE_BUF (4096): это предел атомарной записи в трубу, не выбранное число.
 * Труба O_NONBLOCK: если она полна, событие теряется, а функция возвращает 1.
 * Без зависимостей, кроме libc; потокобезопасна, пока разные потоки используют разные struct s2p_ev. */
#ifndef S2P_EVLINE_H
#define S2P_EVLINE_H

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

#define S2P_STEER_API "1.0"      /* steer-api, под которую собран этот заголовок */
#define S2P_EV_MAX 4096          /* PIPE_BUF */

struct s2p_ev {
    char buf[S2P_EV_MAX];
    size_t n;
    int overflow;                /* строка не поместилась: отправка вернёт -1 */
};

/* Начать событие: {"ev":"<ev>". */
void s2p_ev_begin(struct s2p_ev *e, const char *ev);
/* Поля: значения — строки и целые (docs/steer-api.md, 2: объект плоский). */
void s2p_ev_str(struct s2p_ev *e, const char *key, const char *val);
void s2p_ev_int(struct s2p_ev *e, const char *key, long long val);
/* Закрыть и записать в fd (fd < 0 — взять STEER_EVENT_FD, иначе 3).
 * 0 — записано; 1 — труба полна, событие потеряно (не ошибка); -1 — строка длиннее PIPE_BUF или
 * труба закрыта. */
int s2p_ev_send(struct s2p_ev *e, int fd);

/* Первое событие любого помощника и службы: hello c api = S2P_STEER_API. */
int s2p_ev_hello(const char *ver, const char *mod);
/* Выход готов: up с watch (0/1) и устройством dev (NULL — без dev). */
int s2p_ev_up(int watch, const char *dev);
/* Выход отказал: down с причиной для человека. */
int s2p_ev_down(const char *why);
/* Служба готова. */
int s2p_ev_ready(void);

#ifdef __cplusplus
}
#endif
#endif
