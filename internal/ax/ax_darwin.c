//go:build darwin && cgo

#include "ax_darwin.h"

#include <ApplicationServices/ApplicationServices.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

extern AXError _AXUIElementGetWindow(AXUIElementRef element, CGWindowID *identifier);

struct ax_session {
    AXUIElementRef system_wide;
    AXUIElementRef focused_app;
    AXUIElementRef copy_app;
    CFArrayRef windows;
    int timeout_ms;
};

static bool is_ax_element(CFTypeRef v)
{
    return v && v != kCFNull && CFGetTypeID(v) == AXUIElementGetTypeID();
}

static void session_clear_windows(ax_session_t *s)
{
    if (!s) {
        return;
    }
    if (s->windows) {
        CFRelease(s->windows);
        s->windows = NULL;
    }
    if (s->copy_app) {
        CFRelease(s->copy_app);
        s->copy_app = NULL;
    }
}

void ax_free_windows(ax_window_t *p)
{
    free(p);
}

void ax_session_close(ax_session_t *s)
{
    if (!s) {
        return;
    }
    session_clear_windows(s);
    if (s->focused_app) {
        CFRelease(s->focused_app);
        s->focused_app = NULL;
    }
    if (s->system_wide) {
        CFRelease(s->system_wide);
        s->system_wide = NULL;
    }
    free(s);
}

int ax_is_trusted(bool prompt, bool *trusted_out)
{
    if (!trusted_out) {
        return kAXErrorIllegalArgument;
    }
    Boolean trusted;
    if (prompt) {
        const void *keys[] = { kAXTrustedCheckOptionPrompt };
        const void *vals[] = { kCFBooleanTrue };
        CFDictionaryRef opts = CFDictionaryCreate(
            kCFAllocatorDefault, keys, vals, 1,
            &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
        if (!opts) {
            return kAXErrorFailure;
        }
        trusted = AXIsProcessTrustedWithOptions(opts);
        CFRelease(opts);
    } else {
        trusted = AXIsProcessTrusted();
    }
    *trusted_out = trusted;
    return 0;
}

int ax_session_open(int timeout_ms, ax_session_t **out)
{
    if (!out) {
        return kAXErrorIllegalArgument;
    }
    *out = NULL;
    if (timeout_ms < 50) {
        timeout_ms = 50;
    }

    AXUIElementRef sys = AXUIElementCreateSystemWide();
    if (!sys) {
        return kAXErrorFailure;
    }

    /* Timeout must be set on the system-wide element before any CopyAttributeValue
     * (process default is 6s). */
    float timeout_sec = ((float)timeout_ms) / 1000.0f;
    AXError err = AXUIElementSetMessagingTimeout(sys, timeout_sec);
    if (err != kAXErrorSuccess) {
        CFRelease(sys);
        return (int)err;
    }

    ax_session_t *s = calloc(1, sizeof(*s));
    if (!s) {
        CFRelease(sys);
        return kAXErrorFailure;
    }
    s->system_wide = sys;
    s->timeout_ms = timeout_ms;

    CFTypeRef focused = NULL;
    err = AXUIElementCopyAttributeValue(sys, kAXFocusedApplicationAttribute, &focused);
    if (err == kAXErrorSuccess) {
        if (is_ax_element(focused)) {
            s->focused_app = (AXUIElementRef)focused;
        } else if (focused) {
            CFRelease(focused);
        }
    } else if (err == kAXErrorNoValue) {
        if (focused) {
            CFRelease(focused);
        }
    } else {
        if (focused) {
            CFRelease(focused);
        }
        ax_session_close(s);
        return (int)err;
    }

    *out = s;
    return 0;
}

int ax_session_focused_pid(ax_session_t *s, int32_t *pid_out)
{
    if (!s || !pid_out) {
        return kAXErrorIllegalArgument;
    }
    if (!s->focused_app) {
        return kAXErrorNoValue;
    }
    pid_t pid = 0;
    AXError err = AXUIElementGetPid(s->focused_app, &pid);
    if (err != kAXErrorSuccess) {
        return (int)err;
    }
    if (pid <= 0) {
        return kAXErrorFailure;
    }
    *pid_out = (int32_t)pid;
    return 0;
}

static CFTypeRef ax_val(CFArrayRef values, CFIndex i)
{
    if (!values || i < 0 || i >= CFArrayGetCount(values)) {
        return NULL;
    }
    CFTypeRef v = CFArrayGetValueAtIndex(values, i);
    if (!v || v == kCFNull) {
        return NULL;
    }
    if (CFGetTypeID(v) == AXValueGetTypeID() &&
        AXValueGetType((AXValueRef)v) == kAXValueAXErrorType) {
        return NULL;
    }
    return v;
}

static void copy_str(char *dst, size_t cap, CFTypeRef v)
{
    if (!dst || cap == 0) {
        return;
    }
    dst[0] = 0;
    if (!v || CFGetTypeID(v) != CFStringGetTypeID()) {
        return;
    }
    CFStringRef s = (CFStringRef)v;
    CFIndex used = 0;
    /* CFStringGetBytes does not write a partial UTF-8 code point. */
    CFStringGetBytes(s, CFRangeMake(0, CFStringGetLength(s)),
                     kCFStringEncodingUTF8, 0, false,
                     (UInt8 *)dst, (CFIndex)(cap - 1), &used);
    dst[used] = 0;
}

static bool cf_bool(CFTypeRef v, bool def)
{
    if (!v || CFGetTypeID(v) != CFBooleanGetTypeID()) {
        return def;
    }
    return CFBooleanGetValue((CFBooleanRef)v);
}

static bool ax_cgpoint(CFTypeRef v, CGPoint *out)
{
    if (!out || !v || CFGetTypeID(v) != AXValueGetTypeID()) {
        return false;
    }
    return AXValueGetValue((AXValueRef)v, kAXValueCGPointType, out);
}

static bool ax_cgsize(CFTypeRef v, CGSize *out)
{
    if (!out || !v || CFGetTypeID(v) != AXValueGetTypeID()) {
        return false;
    }
    return AXValueGetValue((AXValueRef)v, kAXValueCGSizeType, out);
}

static CFTypeRef copy_attr(AXUIElementRef el, CFStringRef name)
{
    if (!el || !name) {
        return NULL;
    }
    CFTypeRef v = NULL;
    AXError err = AXUIElementCopyAttributeValue(el, name, &v);
    if (err != kAXErrorSuccess) {
        if (v) {
            CFRelease(v);
        }
        return NULL;
    }
    return v;
}

static void apply_role_subrole_title(ax_window_t *out, CFTypeRef role, CFTypeRef subrole, CFTypeRef title)
{
    copy_str(out->role, sizeof(out->role), role);
    copy_str(out->subrole, sizeof(out->subrole), subrole);
    copy_str(out->title, sizeof(out->title), title);
}

static void apply_frame(ax_window_t *out, CFTypeRef pos, CFTypeRef sz)
{
    CGPoint p;
    if (ax_cgpoint(pos, &p)) {
        out->x = (float)p.x;
        out->y = (float)p.y;
    }
    CGSize s;
    if (ax_cgsize(sz, &s)) {
        out->w = (float)s.width;
        out->h = (float)s.height;
    }
}

static void fill_from_values(CFArrayRef values, ax_window_t *out)
{
    apply_role_subrole_title(out, ax_val(values, 0), ax_val(values, 1), ax_val(values, 2));
    apply_frame(out, ax_val(values, 3), ax_val(values, 4));
    out->minimized = cf_bool(ax_val(values, 5), false);
    out->main = cf_bool(ax_val(values, 6), false);
    out->focused = cf_bool(ax_val(values, 7), false);
}

static void fill_attrs_individual(AXUIElementRef el, ax_window_t *out)
{
    CFTypeRef v;

    v = copy_attr(el, kAXRoleAttribute);
    copy_str(out->role, sizeof(out->role), v);
    if (v) {
        CFRelease(v);
    }

    v = copy_attr(el, kAXSubroleAttribute);
    copy_str(out->subrole, sizeof(out->subrole), v);
    if (v) {
        CFRelease(v);
    }

    v = copy_attr(el, kAXTitleAttribute);
    copy_str(out->title, sizeof(out->title), v);
    if (v) {
        CFRelease(v);
    }

    v = copy_attr(el, kAXPositionAttribute);
    apply_frame(out, v, NULL);
    if (v) {
        CFRelease(v);
    }

    v = copy_attr(el, kAXSizeAttribute);
    apply_frame(out, NULL, v);
    if (v) {
        CFRelease(v);
    }

    v = copy_attr(el, kAXMinimizedAttribute);
    out->minimized = cf_bool(v, false);
    if (v) {
        CFRelease(v);
    }

    v = copy_attr(el, kAXMainAttribute);
    out->main = cf_bool(v, false);
    if (v) {
        CFRelease(v);
    }

    v = copy_attr(el, kAXFocusedAttribute);
    out->focused = cf_bool(v, false);
    if (v) {
        CFRelease(v);
    }
}

static void fill_fullscreen(AXUIElementRef el, ax_window_t *out)
{
    CFTypeRef v = copy_attr(el, CFSTR("AXFullScreen"));
    out->fullscreen = cf_bool(v, false);
    if (v) {
        CFRelease(v);
    }
}

static void fill_window(CFTypeRef raw, int32_t pid, int32_t index, ax_window_t *out)
{
    memset(out, 0, sizeof(*out));
    out->pid = pid;
    out->ax_index = index;
    if (!is_ax_element(raw)) {
        return;
    }
    AXUIElementRef el = (AXUIElementRef)raw;

    CGWindowID wid = 0;
    if (_AXUIElementGetWindow(el, &wid) == kAXErrorSuccess && wid != 0) {
        out->cg_window_id = (uint32_t)wid;
    }

    CFStringRef names[8] = {
        kAXRoleAttribute,
        kAXSubroleAttribute,
        kAXTitleAttribute,
        kAXPositionAttribute,
        kAXSizeAttribute,
        kAXMinimizedAttribute,
        kAXMainAttribute,
        kAXFocusedAttribute,
    };
    CFArrayRef attrs = CFArrayCreate(kCFAllocatorDefault, (const void **)names, 8, &kCFTypeArrayCallBacks);
    CFArrayRef values = NULL;
    AXError err = kAXErrorFailure;
    if (attrs) {
        err = AXUIElementCopyMultipleAttributeValues(el, attrs, 0, &values);
        CFRelease(attrs);
    }
    if (err == kAXErrorSuccess && values) {
        fill_from_values(values, out);
        CFRelease(values);
    } else {
        if (values) {
            CFRelease(values);
        }
        fill_attrs_individual(el, out);
    }

    fill_fullscreen(el, out);
}

int ax_session_copy_windows(ax_session_t *s, int32_t pid, ax_window_t **out, int *n_out)
{
    if (!s || !out || !n_out) {
        return kAXErrorIllegalArgument;
    }
    *out = NULL;
    *n_out = 0;

    AXUIElementRef app = AXUIElementCreateApplication((pid_t)pid);
    if (!app) {
        return kAXErrorFailure;
    }

    CFTypeRef wins = NULL;
    AXError err = AXUIElementCopyAttributeValue(app, kAXWindowsAttribute, &wins);
    if (err == kAXErrorAttributeUnsupported || err == kAXErrorNoValue) {
        session_clear_windows(s);
        s->copy_app = app;
        return 0;
    }
    if (err != kAXErrorSuccess) {
        if (wins) {
            CFRelease(wins);
        }
        CFRelease(app);
        return (int)err;
    }
    if (!wins) {
        session_clear_windows(s);
        s->copy_app = app;
        return 0;
    }
    if (CFGetTypeID(wins) != CFArrayGetTypeID()) {
        CFRelease(wins);
        CFRelease(app);
        return kAXErrorFailure;
    }

    CFArrayRef arr = (CFArrayRef)wins;
    int n = (int)CFArrayGetCount(arr);
    ax_window_t *buf = NULL;
    if (n > 0) {
        buf = calloc((size_t)n, sizeof(*buf));
        if (!buf) {
            CFRelease(arr);
            CFRelease(app);
            return kAXErrorFailure;
        }
        for (int i = 0; i < n; i++) {
            fill_window(CFArrayGetValueAtIndex(arr, i), pid, i, &buf[i]);
        }
    }

    session_clear_windows(s);
    s->copy_app = app;
    s->windows = arr;
    *out = buf;
    *n_out = n;
    return 0;
}

int ax_session_focused_window_id(ax_session_t *s, int32_t pid, uint32_t *cg_out, ax_window_t *meta_out)
{
    if (!s || !cg_out) {
        return kAXErrorIllegalArgument;
    }
    *cg_out = 0;
    if (meta_out) {
        memset(meta_out, 0, sizeof(*meta_out));
        meta_out->pid = pid;
    }

    AXUIElementRef app = AXUIElementCreateApplication((pid_t)pid);
    if (!app) {
        return kAXErrorFailure;
    }

    CFTypeRef focused = NULL;
    AXError err = AXUIElementCopyAttributeValue(app, kAXFocusedWindowAttribute, &focused);
    CFRelease(app);
    if (err == kAXErrorAttributeUnsupported || err == kAXErrorNoValue || err == kAXErrorInvalidUIElement) {
        if (focused) {
            CFRelease(focused);
        }
        return 0;
    }
    if (err != kAXErrorSuccess) {
        if (focused) {
            CFRelease(focused);
        }
        return (int)err;
    }
    if (!is_ax_element(focused)) {
        if (focused) {
            CFRelease(focused);
        }
        return 0;
    }

    AXUIElementRef el = (AXUIElementRef)focused;
    ax_window_t meta;
    /* AXIndex is unknown here; callers must use the Windows() snapshot index to raise. */
    fill_window(el, pid, -1, &meta);
    *cg_out = meta.cg_window_id;
    if (meta_out) {
        *meta_out = meta;
    }
    CFRelease(el);
    return 0;
}

static bool window_minimized(AXUIElementRef w)
{
    CFTypeRef v = copy_attr(w, kAXMinimizedAttribute);
    bool mini = cf_bool(v, false);
    if (v) {
        CFRelease(v);
    }
    return mini;
}

static AXError retry_cannot_complete(AXError err, AXError (*fn)(AXUIElementRef), AXUIElementRef w)
{
    if (err != kAXErrorCannotComplete) {
        return err;
    }
    usleep(30000);
    return fn(w);
}

static AXError perform_raise(AXUIElementRef w)
{
    return AXUIElementPerformAction(w, kAXRaiseAction);
}

static AXError set_main_true(AXUIElementRef w)
{
    return AXUIElementSetAttributeValue(w, kAXMainAttribute, kCFBooleanTrue);
}

int ax_session_raise_index(ax_session_t *s, int index, bool unminimize, bool do_raise, bool set_main)
{
    if (!s || !s->windows) {
        return kAXErrorIllegalArgument;
    }
    CFIndex n = CFArrayGetCount(s->windows);
    if (index < 0 || (CFIndex)index >= n) {
        return kAXErrorIllegalArgument;
    }
    CFTypeRef item = CFArrayGetValueAtIndex(s->windows, (CFIndex)index);
    if (!is_ax_element(item)) {
        return kAXErrorInvalidUIElement;
    }
    AXUIElementRef w = (AXUIElementRef)item;

    if (unminimize) {
        (void)AXUIElementSetAttributeValue(w, kAXMinimizedAttribute, kCFBooleanFalse);
        int waited = 0;
        while (waited < s->timeout_ms) {
            if (!window_minimized(w)) {
                break;
            }
            usleep(10000);
            waited += 10;
        }
    }

    AXError err = kAXErrorSuccess;
    if (do_raise) {
        err = perform_raise(w);
        err = retry_cannot_complete(err, perform_raise, w);
        if (err != kAXErrorSuccess) {
            return (int)err;
        }
    }
    if (set_main) {
        err = set_main_true(w);
        err = retry_cannot_complete(err, set_main_true, w);
        if (err != kAXErrorSuccess) {
            return (int)err;
        }
    }
    return 0;
}
