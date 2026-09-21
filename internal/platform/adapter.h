#ifndef MISTERVISION_ADAPTER_H
#define MISTERVISION_ADAPTER_H
#include <stddef.h>
#include <stdint.h>
typedef struct mf_display mf_display;
/* Returns an errno value, or zero. C never retains the input pixel pointer. */
int mf_open(mf_display **out, const char *device, int width, int height);
void mf_set_aspect(mf_display *d, double aspect);
void mf_geometry(mf_display *d, int *w, int *h, int *ow, int *oh);
void mf_raster_size(mf_display *d, int *w, int *h);
int mf_present_raster(mf_display *d, const uint8_t *pixels, size_t size, int width, int height);
int mf_present(mf_display *d, const uint8_t *pixels, size_t size);
int mf_present_full_raster(mf_display *d, const uint8_t *pixels, size_t size, int width, int height);
int mf_present_video(mf_display *d, const uint8_t *pixels, size_t size);
int mf_dump(mf_display *d, const char *path);
int mf_close(mf_display *d);
#endif
