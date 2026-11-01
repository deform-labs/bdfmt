package opencl

/*
#cgo LDFLAGS: -ldl
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>
#include <dlfcn.h>
#include <stdint.h>

typedef int32_t cl_int;
typedef uint32_t cl_uint;
typedef uint64_t cl_ulong;
typedef uint64_t cl_bitfield;
typedef cl_bitfield cl_device_type;
typedef cl_bitfield cl_mem_flags;
typedef cl_bitfield cl_command_queue_properties;

typedef void* cl_platform_id;
typedef void* cl_device_id;
typedef void* cl_context;
typedef void* cl_command_queue;
typedef void* cl_mem;
typedef void* cl_program;
typedef void* cl_kernel;
typedef void* cl_event;

#define CL_SUCCESS 0
#define CL_DEVICE_TYPE_GPU (1 << 2)
#define CL_DEVICE_TYPE_ALL 0xFFFFFFFF

#define CL_DEVICE_NAME 0x102B
#define CL_DEVICE_VENDOR 0x102C
#define CL_DEVICE_TYPE 0x1000
#define CL_DEVICE_MAX_COMPUTE_UNITS 0x1002
#define CL_DEVICE_MAX_CLOCK_FREQUENCY 0x100C
#define CL_DEVICE_GLOBAL_MEM_SIZE 0x101F

#define CL_MEM_READ_WRITE (1 << 0)
#define CL_MEM_WRITE_ONLY (1 << 1)
#define CL_MEM_READ_ONLY  (1 << 2)
#define CL_MEM_COPY_HOST_PTR (1 << 5)

#define CL_PROGRAM_BUILD_LOG 0x1183

typedef struct {
    void* handle;
    cl_platform_id platform;
    cl_device_id device;
    cl_context context;
    cl_command_queue queue;
    cl_program program;
    cl_kernel kernel;
    cl_kernel noiseKernel;
    char deviceName[256];
    char vendorName[256];
    uint32_t computeUnits;
    uint32_t clockFreq;
    uint64_t globalMemSize;
} CLContext;

typedef struct {
    int32_t x;
    int32_t y;
} Int2;

// Function pointers
typedef cl_int (*pfn_clGetPlatformIDs)(cl_uint, cl_platform_id*, cl_uint*);
typedef cl_int (*pfn_clGetDeviceIDs)(cl_platform_id, cl_device_type, cl_uint, cl_device_id*, cl_uint*);
typedef cl_int (*pfn_clGetDeviceInfo)(cl_device_id, cl_uint, size_t, void*, size_t*);
typedef cl_context (*pfn_clCreateContext)(const void*, cl_uint, const cl_device_id*, void*, void*, cl_int*);
typedef cl_int (*pfn_clReleaseContext)(cl_context);
typedef cl_command_queue (*pfn_clCreateCommandQueue)(cl_context, cl_device_id, cl_command_queue_properties, cl_int*);
typedef cl_int (*pfn_clReleaseCommandQueue)(cl_command_queue);
typedef cl_program (*pfn_clCreateProgramWithSource)(cl_context, cl_uint, const char**, const size_t*, cl_int*);
typedef cl_int (*pfn_clBuildProgram)(cl_program, cl_uint, const cl_device_id*, const char*, void*, void*);
typedef cl_int (*pfn_clGetProgramBuildInfo)(cl_program, cl_device_id, cl_uint, size_t, void*, size_t*);
typedef cl_int (*pfn_clReleaseProgram)(cl_program);
typedef cl_kernel (*pfn_clCreateKernel)(cl_program, const char*, cl_int*);
typedef cl_int (*pfn_clSetKernelArg)(cl_kernel, cl_uint, size_t, const void*);
typedef cl_int (*pfn_clReleaseKernel)(cl_kernel);
typedef cl_mem (*pfn_clCreateBuffer)(cl_context, cl_mem_flags, size_t, void*, cl_int*);
typedef cl_int (*pfn_clReleaseMemObject)(cl_mem);
typedef cl_int (*pfn_clEnqueueWriteBuffer)(cl_command_queue, cl_mem, cl_uint, size_t, size_t, const void*, cl_uint, const cl_event*, cl_event*);
typedef cl_int (*pfn_clEnqueueNDRangeKernel)(cl_command_queue, cl_kernel, cl_uint, const size_t*, const size_t*, const size_t*, cl_uint, const cl_event*, cl_event*);
typedef cl_int (*pfn_clEnqueueReadBuffer)(cl_command_queue, cl_mem, cl_uint, size_t, size_t, void*, cl_uint, const cl_event*, cl_event*);
typedef cl_int (*pfn_clFinish)(cl_command_queue);

static pfn_clGetPlatformIDs clGetPlatformIDs_p = NULL;
static pfn_clGetDeviceIDs clGetDeviceIDs_p = NULL;
static pfn_clGetDeviceInfo clGetDeviceInfo_p = NULL;
static pfn_clCreateContext clCreateContext_p = NULL;
static pfn_clReleaseContext clReleaseContext_p = NULL;
static pfn_clCreateCommandQueue clCreateCommandQueue_p = NULL;
static pfn_clReleaseCommandQueue clReleaseCommandQueue_p = NULL;
static pfn_clCreateProgramWithSource clCreateProgramWithSource_p = NULL;
static pfn_clBuildProgram clBuildProgram_p = NULL;
static pfn_clGetProgramBuildInfo clGetProgramBuildInfo_p = NULL;
static pfn_clReleaseProgram clReleaseProgram_p = NULL;
static pfn_clCreateKernel clCreateKernel_p = NULL;
static pfn_clSetKernelArg clSetKernelArg_p = NULL;
static pfn_clReleaseKernel clReleaseKernel_p = NULL;
static pfn_clCreateBuffer clCreateBuffer_p = NULL;
static pfn_clReleaseMemObject clReleaseMemObject_p = NULL;
static pfn_clEnqueueWriteBuffer clEnqueueWriteBuffer_p = NULL;
static pfn_clEnqueueNDRangeKernel clEnqueueNDRangeKernel_p = NULL;
static pfn_clEnqueueReadBuffer clEnqueueReadBuffer_p = NULL;
static pfn_clFinish clFinish_p = NULL;

static int load_opencl_symbols(void* handle) {
    #define LOAD_SYM(name) name##_p = (pfn_##name)dlsym(handle, #name); if (!name##_p) return -1;
    LOAD_SYM(clGetPlatformIDs);
    LOAD_SYM(clGetDeviceIDs);
    LOAD_SYM(clGetDeviceInfo);
    LOAD_SYM(clCreateContext);
    LOAD_SYM(clReleaseContext);
    LOAD_SYM(clCreateCommandQueue);
    LOAD_SYM(clReleaseCommandQueue);
    LOAD_SYM(clCreateProgramWithSource);
    LOAD_SYM(clBuildProgram);
    LOAD_SYM(clGetProgramBuildInfo);
    LOAD_SYM(clReleaseProgram);
    LOAD_SYM(clCreateKernel);
    LOAD_SYM(clSetKernelArg);
    LOAD_SYM(clReleaseKernel);
    LOAD_SYM(clCreateBuffer);
    LOAD_SYM(clReleaseMemObject);
    LOAD_SYM(clEnqueueWriteBuffer);
    LOAD_SYM(clEnqueueNDRangeKernel);
    LOAD_SYM(clEnqueueReadBuffer);
    LOAD_SYM(clFinish);
    #undef LOAD_SYM
    return 0;
}

static int device_name_matches(const char* name, const char* preference) {
    if (!preference || !preference[0]) return 1;
    size_t n = strlen(preference);
    for (const char* p = name; *p; p++) {
        size_t i = 0;
        while (i < n && p[i] && tolower((unsigned char)p[i]) == tolower((unsigned char)preference[i])) i++;
        if (i == n) return 1;
    }
    return 0;
}

static CLContext* init_opencl_gpu(const char* kernelSrc, const char* preference, char* errorBuf, int errorBufLen) {
    void* handle = dlopen("libOpenCL.so.1", RTLD_LAZY);
    if (!handle) {
        handle = dlopen("libOpenCL.so", RTLD_LAZY);
    }
    if (!handle) {
        snprintf(errorBuf, errorBufLen, "libOpenCL.so not found");
        return NULL;
    }

    if (load_opencl_symbols(handle) != 0) {
        snprintf(errorBuf, errorBufLen, "failed to load OpenCL symbols");
        dlclose(handle);
        return NULL;
    }

    cl_uint numPlatforms = 0;
    cl_int res = clGetPlatformIDs_p(0, NULL, &numPlatforms);
    if (res != CL_SUCCESS || numPlatforms == 0) {
        snprintf(errorBuf, errorBufLen, "no OpenCL platforms found (clGetPlatformIDs code %d)", res);
        dlclose(handle);
        return NULL;
    }

    cl_platform_id* platforms = (cl_platform_id*)malloc(numPlatforms * sizeof(cl_platform_id));
    clGetPlatformIDs_p(numPlatforms, platforms, NULL);

    cl_device_id gpuDevice = NULL;
    cl_platform_id targetPlatform = NULL;
    cl_device_type searchTypes[2] = {CL_DEVICE_TYPE_GPU, CL_DEVICE_TYPE_ALL};
    for (int pass = 0; pass < 2 && !gpuDevice; pass++) {
        for (cl_uint i = 0; i < numPlatforms && !gpuDevice; i++) {
            cl_uint numDevices = 0;
            if (clGetDeviceIDs_p(platforms[i], searchTypes[pass], 0, NULL, &numDevices) != CL_SUCCESS || numDevices == 0) continue;
            cl_device_id* devices = (cl_device_id*)malloc(numDevices * sizeof(cl_device_id));
            if (!devices) continue;
            if (clGetDeviceIDs_p(platforms[i], searchTypes[pass], numDevices, devices, NULL) == CL_SUCCESS) {
                for (cl_uint j = 0; j < numDevices; j++) {
                    char name[256] = {0};
                    clGetDeviceInfo_p(devices[j], CL_DEVICE_NAME, sizeof(name), name, NULL);
                    if (device_name_matches(name, preference)) {
                        gpuDevice = devices[j];
                        targetPlatform = platforms[i];
                        break;
                    }
                }
            }
            free(devices);
        }
    }
    free(platforms);

    if (!gpuDevice) {
        if (preference && preference[0]) snprintf(errorBuf, errorBufLen, "no OpenCL device matches '%s'", preference);
        else snprintf(errorBuf, errorBufLen, "no OpenCL devices found on any platform");
        dlclose(handle);
        return NULL;
    }

    CLContext* ctx = (CLContext*)calloc(1, sizeof(CLContext));
    ctx->handle = handle;
    ctx->platform = targetPlatform;
    ctx->device = gpuDevice;

    clGetDeviceInfo_p(gpuDevice, CL_DEVICE_NAME, sizeof(ctx->deviceName), ctx->deviceName, NULL);
    clGetDeviceInfo_p(gpuDevice, CL_DEVICE_VENDOR, sizeof(ctx->vendorName), ctx->vendorName, NULL);
    clGetDeviceInfo_p(gpuDevice, CL_DEVICE_MAX_COMPUTE_UNITS, sizeof(ctx->computeUnits), &ctx->computeUnits, NULL);
    clGetDeviceInfo_p(gpuDevice, CL_DEVICE_MAX_CLOCK_FREQUENCY, sizeof(ctx->clockFreq), &ctx->clockFreq, NULL);
    clGetDeviceInfo_p(gpuDevice, CL_DEVICE_GLOBAL_MEM_SIZE, sizeof(ctx->globalMemSize), &ctx->globalMemSize, NULL);

    cl_int err = 0;
    ctx->context = clCreateContext_p(NULL, 1, &gpuDevice, NULL, NULL, &err);
    if (err != CL_SUCCESS || !ctx->context) {
        snprintf(errorBuf, errorBufLen, "clCreateContext failed with code %d", err);
        free(ctx);
        dlclose(handle);
        return NULL;
    }

    ctx->queue = clCreateCommandQueue_p(ctx->context, gpuDevice, 0, &err);
    if (err != CL_SUCCESS || !ctx->queue) {
        snprintf(errorBuf, errorBufLen, "clCreateCommandQueue failed with code %d", err);
        clReleaseContext_p(ctx->context);
        free(ctx);
        dlclose(handle);
        return NULL;
    }

    size_t srcLen = strlen(kernelSrc);
    ctx->program = clCreateProgramWithSource_p(ctx->context, 1, &kernelSrc, &srcLen, &err);
    if (err != CL_SUCCESS || !ctx->program) {
        snprintf(errorBuf, errorBufLen, "clCreateProgramWithSource failed: %d", err);
        clReleaseCommandQueue_p(ctx->queue);
        clReleaseContext_p(ctx->context);
        free(ctx);
        dlclose(handle);
        return NULL;
    }

    err = clBuildProgram_p(ctx->program, 1, &gpuDevice, "-cl-fast-relaxed-math", NULL, NULL);
    if (err != CL_SUCCESS) {
        char buildLog[4096] = {0};
        clGetProgramBuildInfo_p(ctx->program, gpuDevice, CL_PROGRAM_BUILD_LOG, sizeof(buildLog), buildLog, NULL);
        snprintf(errorBuf, errorBufLen, "clBuildProgram failed: %s", buildLog);
        clReleaseProgram_p(ctx->program);
        clReleaseCommandQueue_p(ctx->queue);
        clReleaseContext_p(ctx->context);
        free(ctx);
        dlclose(handle);
        return NULL;
    }

    ctx->kernel = clCreateKernel_p(ctx->program, "generate_chunk_voxels", &err);
    if (err != CL_SUCCESS || !ctx->kernel) {
        snprintf(errorBuf, errorBufLen, "clCreateKernel failed: %d", err);
        clReleaseProgram_p(ctx->program);
        clReleaseCommandQueue_p(ctx->queue);
        clReleaseContext_p(ctx->context);
        free(ctx);
        dlclose(handle);
        return NULL;
    }

    ctx->noiseKernel = clCreateKernel_p(ctx->program, "generate_chunk_heights", &err);
    if (err != CL_SUCCESS || !ctx->noiseKernel) {
        snprintf(errorBuf, errorBufLen, "clCreateKernel for height map failed: %d", err);
        clReleaseKernel_p(ctx->kernel);
        clReleaseProgram_p(ctx->program);
        clReleaseCommandQueue_p(ctx->queue);
        clReleaseContext_p(ctx->context);
        free(ctx);
        dlclose(handle);
        return NULL;
    }

    return ctx;
}

static int list_opencl_devices(char* output, int outputLen, char* errorBuf, int errorBufLen) {
    void* handle = dlopen("libOpenCL.so.1", RTLD_LAZY);
    if (!handle) handle = dlopen("libOpenCL.so", RTLD_LAZY);
    if (!handle) { snprintf(errorBuf, errorBufLen, "libOpenCL.so not found"); return -1; }
    if (load_opencl_symbols(handle) != 0) { snprintf(errorBuf, errorBufLen, "failed to load OpenCL symbols"); dlclose(handle); return -1; }
    cl_uint numPlatforms = 0;
    cl_int result = clGetPlatformIDs_p(0, NULL, &numPlatforms);
    if (result != CL_SUCCESS || numPlatforms == 0) { snprintf(errorBuf, errorBufLen, "no OpenCL platforms found"); dlclose(handle); return -1; }
    cl_platform_id* platforms = (cl_platform_id*)malloc(numPlatforms * sizeof(cl_platform_id));
    if (!platforms) { snprintf(errorBuf, errorBufLen, "out of memory"); dlclose(handle); return -1; }
    clGetPlatformIDs_p(numPlatforms, platforms, NULL);
    int used = 0, count = 0;
    for (cl_uint i = 0; i < numPlatforms; i++) {
        cl_uint numDevices = 0;
        if (clGetDeviceIDs_p(platforms[i], CL_DEVICE_TYPE_ALL, 0, NULL, &numDevices) != CL_SUCCESS || !numDevices) continue;
        cl_device_id* devices = (cl_device_id*)malloc(numDevices * sizeof(cl_device_id));
        if (!devices) continue;
        if (clGetDeviceIDs_p(platforms[i], CL_DEVICE_TYPE_ALL, numDevices, devices, NULL) == CL_SUCCESS) {
            for (cl_uint j = 0; j < numDevices; j++) {
                char name[256] = {0}, vendor[256] = {0};
                cl_bitfield type = 0;
                clGetDeviceInfo_p(devices[j], CL_DEVICE_NAME, sizeof(name), name, NULL);
                clGetDeviceInfo_p(devices[j], CL_DEVICE_VENDOR, sizeof(vendor), vendor, NULL);
                clGetDeviceInfo_p(devices[j], CL_DEVICE_TYPE, sizeof(type), &type, NULL);
                int written = snprintf(output + used, outputLen > used ? outputLen - used : 0,
                    "%d. [%s] %s — %s\n", count++, (type & CL_DEVICE_TYPE_GPU) ? "GPU" : "CPU/Other", vendor, name);
                if (written > 0) used += written;
            }
        }
        free(devices);
    }
    free(platforms);
    dlclose(handle);
    return count;
}

static int run_cl_chunk_voxels(
    CLContext* ctx,
    int numChunks,
    const Int2* chunkCoords,
    float frequency,
    float seaLevel,
    float baseHeight,
    float heightScale,
    int octaves,
    float persistence,
    float lacunarity,
    float maximumSurfaceHeight,
    const int* perm,
    uint8_t* outSectionTypes,
    uint64_t* outSectionData
) {
    if (!ctx || !ctx->kernel || !ctx->noiseKernel || !ctx->queue) return -1;

    cl_int err = 0;
    size_t totalWords = (size_t)numChunks * 24 * 256;
    size_t totalTypes = (size_t)numChunks * 24;
    size_t totalHeights = (size_t)numChunks * 256;

    cl_mem coordsBuf = clCreateBuffer_p(ctx->context, CL_MEM_READ_ONLY | CL_MEM_COPY_HOST_PTR, numChunks * sizeof(Int2), (void*)chunkCoords, &err);
    if (err != CL_SUCCESS) return err;

    cl_mem permBuf = clCreateBuffer_p(ctx->context, CL_MEM_READ_ONLY | CL_MEM_COPY_HOST_PTR, 512 * sizeof(int), (void*)perm, &err);
    if (err != CL_SUCCESS) {
        clReleaseMemObject_p(coordsBuf);
        return err;
    }

    cl_mem heightsBuf = clCreateBuffer_p(ctx->context, CL_MEM_READ_WRITE, totalHeights * sizeof(int32_t), NULL, &err);
    if (err != CL_SUCCESS) {
        clReleaseMemObject_p(coordsBuf);
        clReleaseMemObject_p(permBuf);
        return err;
    }

    cl_mem typesBuf = clCreateBuffer_p(ctx->context, CL_MEM_WRITE_ONLY, totalTypes * sizeof(uint8_t), NULL, &err);
    if (err != CL_SUCCESS) {
        clReleaseMemObject_p(coordsBuf);
        clReleaseMemObject_p(permBuf);
        clReleaseMemObject_p(heightsBuf);
        return err;
    }

    cl_mem dataBuf = clCreateBuffer_p(ctx->context, CL_MEM_WRITE_ONLY, totalWords * sizeof(uint64_t), NULL, &err);
    if (err != CL_SUCCESS) {
        clReleaseMemObject_p(coordsBuf);
        clReleaseMemObject_p(permBuf);
        clReleaseMemObject_p(heightsBuf);
        clReleaseMemObject_p(typesBuf);
        return err;
    }

    // First compute each chunk's 16x16 height map once.
    clSetKernelArg_p(ctx->noiseKernel, 0, sizeof(int), &numChunks);
    clSetKernelArg_p(ctx->noiseKernel, 1, sizeof(cl_mem), &coordsBuf);
    clSetKernelArg_p(ctx->noiseKernel, 2, sizeof(float), &frequency);
    clSetKernelArg_p(ctx->noiseKernel, 3, sizeof(float), &seaLevel);
    clSetKernelArg_p(ctx->noiseKernel, 4, sizeof(float), &baseHeight);
    clSetKernelArg_p(ctx->noiseKernel, 5, sizeof(float), &heightScale);
    clSetKernelArg_p(ctx->noiseKernel, 6, sizeof(int), &octaves);
    clSetKernelArg_p(ctx->noiseKernel, 7, sizeof(float), &persistence);
    clSetKernelArg_p(ctx->noiseKernel, 8, sizeof(float), &lacunarity);
    clSetKernelArg_p(ctx->noiseKernel, 9, sizeof(cl_mem), &permBuf);
    clSetKernelArg_p(ctx->noiseKernel, 10, sizeof(cl_mem), &heightsBuf);
    size_t heightWorkSize[3] = {16, 16, (size_t)numChunks};
    err = clEnqueueNDRangeKernel_p(ctx->queue, ctx->noiseKernel, 3, NULL, heightWorkSize, NULL, 0, NULL, NULL);

    // Then build all chunk section rows in parallel from the cached heights.
    clSetKernelArg_p(ctx->kernel, 0, sizeof(int), &numChunks);
    clSetKernelArg_p(ctx->kernel, 1, sizeof(float), &seaLevel);
    clSetKernelArg_p(ctx->kernel, 2, sizeof(float), &maximumSurfaceHeight);
    clSetKernelArg_p(ctx->kernel, 3, sizeof(cl_mem), &heightsBuf);
    clSetKernelArg_p(ctx->kernel, 4, sizeof(cl_mem), &typesBuf);
    clSetKernelArg_p(ctx->kernel, 5, sizeof(cl_mem), &dataBuf);
    size_t chunkWorkSize[3] = {256, 24, (size_t)numChunks};
    if (err == CL_SUCCESS) {
        err = clEnqueueNDRangeKernel_p(ctx->queue, ctx->kernel, 3, NULL, chunkWorkSize, NULL, 0, NULL, NULL);
    }
    if (err != CL_SUCCESS) {
        clReleaseMemObject_p(coordsBuf);
        clReleaseMemObject_p(permBuf);
        clReleaseMemObject_p(heightsBuf);
        clReleaseMemObject_p(typesBuf);
        clReleaseMemObject_p(dataBuf);
        return err;
    }

    // Read back results into host buffers
    clEnqueueReadBuffer_p(ctx->queue, typesBuf, 0, 0, totalTypes * sizeof(uint8_t), outSectionTypes, 0, NULL, NULL);
    clEnqueueReadBuffer_p(ctx->queue, dataBuf, 1, 0, totalWords * sizeof(uint64_t), outSectionData, 0, NULL, NULL);
    clFinish_p(ctx->queue);

    clReleaseMemObject_p(coordsBuf);
    clReleaseMemObject_p(permBuf);
    clReleaseMemObject_p(heightsBuf);
    clReleaseMemObject_p(typesBuf);
    clReleaseMemObject_p(dataBuf);

    return CL_SUCCESS;
}

static void release_cl_context(CLContext* ctx) {
    if (!ctx) return;
    if (ctx->noiseKernel) clReleaseKernel_p(ctx->noiseKernel);
    if (ctx->kernel) clReleaseKernel_p(ctx->kernel);
    if (ctx->program) clReleaseProgram_p(ctx->program);
    if (ctx->queue) clReleaseCommandQueue_p(ctx->queue);
    if (ctx->context) clReleaseContext_p(ctx->context);
    if (ctx->handle) dlclose(ctx->handle);
    free(ctx);
}
*/
import "C"

import (
	"fmt"
	"sync"
	"unsafe"
)

// DeviceInfo contains OpenCL hardware specifications
type DeviceInfo struct {
	DeviceName     string
	VendorName     string
	ComputeUnits   uint32
	ClockFrequency uint32
	GlobalMemSize  uint64
}

// Engine wraps OpenCL GPU compute pipeline
type Engine struct {
	mu   sync.Mutex
	ctx  *C.CLContext
	info DeviceInfo
}

// InitGPU initializes OpenCL GPU compute context
func InitGPU(preference ...string) (*Engine, error) {
	devicePreference := ""
	if len(preference) > 0 {
		devicePreference = preference[0]
	}
	cKernel := C.CString(KernelSource)
	defer C.free(unsafe.Pointer(cKernel))
	cPreference := C.CString(devicePreference)
	defer C.free(unsafe.Pointer(cPreference))

	errBuf := make([]byte, 1024)
	cCtx := C.init_opencl_gpu(cKernel, cPreference, (*C.char)(unsafe.Pointer(&errBuf[0])), C.int(len(errBuf)))
	if cCtx == nil {
		errStr := C.GoString((*C.char)(unsafe.Pointer(&errBuf[0])))
		return nil, fmt.Errorf("opencl init error: %s", errStr)
	}

	info := DeviceInfo{
		DeviceName:     C.GoString(&cCtx.deviceName[0]),
		VendorName:     C.GoString(&cCtx.vendorName[0]),
		ComputeUnits:   uint32(cCtx.computeUnits),
		ClockFrequency: uint32(cCtx.clockFreq),
		GlobalMemSize:  uint64(cCtx.globalMemSize),
	}

	return &Engine{
		ctx:  cCtx,
		info: info,
	}, nil
}

// ListDevices returns the available OpenCL devices and their types.
func ListDevices() (string, error) {
	output := make([]byte, 8192)
	errBuf := make([]byte, 1024)
	count := C.list_opencl_devices((*C.char)(unsafe.Pointer(&output[0])), C.int(len(output)), (*C.char)(unsafe.Pointer(&errBuf[0])), C.int(len(errBuf)))
	if count < 0 {
		return "", fmt.Errorf("opencl device listing error: %s", C.GoString((*C.char)(unsafe.Pointer(&errBuf[0]))))
	}
	if count == 0 {
		return "No OpenCL devices found.\n", nil
	}
	return C.GoString((*C.char)(unsafe.Pointer(&output[0]))), nil
}

// Info returns device information
func (e *Engine) Info() DeviceInfo {
	return e.info
}

// Close releases OpenCL resources
func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.ctx != nil {
		C.release_cl_context(e.ctx)
		e.ctx = nil
	}
}

// GPUChunkResult holds the GPU-generated section data for one chunk
type GPUChunkResult struct {
	ChunkX       int
	ChunkZ       int
	SectionTypes [24]uint8
	SectionData  []int64 // packed as 24 consecutive arrays of 256 longs
}

// GenerateChunkVoxelsBatch dispatches GPU compute to generate full 3D voxel sections for a batch of chunks
func (e *Engine) GenerateChunkVoxelsBatch(
	coords [][2]int,
	frequency float32,
	seaLevel float32,
	baseHeight float32,
	heightScale float32,
	octaves int,
	persistence float32,
	lacunarity float32,
	maximumSurfaceHeight float32,
	perm [512]int,
) ([]GPUChunkResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.ctx == nil {
		return nil, fmt.Errorf("opencl context is closed")
	}

	numChunks := len(coords)
	if numChunks == 0 {
		return nil, nil
	}

	cCoords := make([]C.Int2, numChunks)
	for i, c := range coords {
		cCoords[i] = C.Int2{x: C.int32_t(c[0]), y: C.int32_t(c[1])}
	}

	totalTypes := numChunks * 24
	totalWords := numChunks * 24 * 256

	outTypes := make([]uint8, totalTypes)
	outData := make([]uint64, totalWords)

	cPerm := make([]C.int, 512)
	for i := 0; i < 512; i++ {
		cPerm[i] = C.int(perm[i])
	}

	res := C.run_cl_chunk_voxels(
		e.ctx,
		C.int(numChunks),
		&cCoords[0],
		C.float(frequency),
		C.float(seaLevel),
		C.float(baseHeight),
		C.float(heightScale),
		C.int(octaves),
		C.float(persistence),
		C.float(lacunarity),
		C.float(maximumSurfaceHeight),
		&cPerm[0],
		(*C.uint8_t)(unsafe.Pointer(&outTypes[0])),
		(*C.uint64_t)(unsafe.Pointer(&outData[0])),
	)

	if res != C.CL_SUCCESS {
		return nil, fmt.Errorf("opencl chunk voxel kernel failed with code: %d", int(res))
	}

	results := make([]GPUChunkResult, numChunks)
	for c := 0; c < numChunks; c++ {
		results[c].ChunkX = coords[c][0]
		results[c].ChunkZ = coords[c][1]
		for s := 0; s < 24; s++ {
			results[c].SectionTypes[s] = outTypes[c*24+s]
		}
		dataOffset := c * 24 * 256
		results[c].SectionData = unsafe.Slice((*int64)(unsafe.Pointer(&outData[dataOffset])), 24*256)
	}

	return results, nil
}
