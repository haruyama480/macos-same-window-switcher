#pragma once
#include <stdint.h>
#include <stdbool.h>

typedef struct ax_session ax_session_t;

typedef struct {
    uint32_t cg_window_id; // 0 = unavailable
    int32_t  pid;
    int32_t  ax_index;
    char     role[64];
    char     subrole[64];
    char     title[512]; // UTF-8, truncate on code-point boundary
    float    x, y, w, h;
    bool     minimized;
    bool     fullscreen;
    bool     main;
    bool     focused;
} ax_window_t;

int ax_is_trusted(bool prompt, bool *trusted_out); // 0 = check succeeded
int ax_session_open(int timeout_ms, ax_session_t **out); // CreateSystemWide, SetMessagingTimeout FIRST, then focused app. clamp timeout_ms >= 50
int ax_session_focused_pid(ax_session_t *s, int32_t *pid_out);
int ax_session_copy_windows(ax_session_t *s, int32_t pid, ax_window_t **out, int *n_out);
int ax_session_focused_window_id(ax_session_t *s, int32_t pid, uint32_t *cg_out, ax_window_t *meta_out);
int ax_session_raise_index(ax_session_t *s, int index, bool unminimize, bool do_raise, bool set_main);
void ax_free_windows(ax_window_t *p);
void ax_session_close(ax_session_t *s);
