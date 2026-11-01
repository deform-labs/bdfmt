package main

/*
#include <stdint.h>
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/twlve-secs/bdfmt/pkg/bdf"
)

func readRoot(data *C.uchar, length C.int) (*bdf.TagCompound, error) {
	if length < 0 || (length > 0 && data == nil) {
		return nil, fmt.Errorf("invalid input buffer")
	}
	var bytes []byte
	if length > 0 {
		bytes = unsafe.Slice((*byte)(unsafe.Pointer(data)), int(length))
	}
	tag, err := bdf.Read(bytes)
	if err != nil {
		return nil, err
	}
	root, ok := tag.(*bdf.TagCompound)
	if !ok {
		return nil, fmt.Errorf("root tag is not a compound")
	}
	return root, nil
}

//export BDFNativeValidate
func BDFNativeValidate(data *C.uchar, length C.int) (result C.int) {
	defer func() {
		if recover() != nil {
			result = -1
		}
	}()
	if _, err := readRoot(data, length); err != nil {
		return -1
	}
	return 0
}

//export BDFNativeGetInt
func BDFNativeGetInt(data *C.uchar, length C.int, keyData *C.uchar, keyLength C.int, output *C.int32_t) (result C.int) {
	defer func() {
		if recover() != nil {
			result = -1
		}
	}()
	root, err := readRoot(data, length)
	if err != nil || keyLength < 0 || (keyLength > 0 && keyData == nil) {
		return -1
	}
	key := ""
	if keyLength > 0 {
		key = string(unsafe.Slice((*byte)(unsafe.Pointer(keyData)), int(keyLength)))
	}
	value, ok := root.Get(key)
	if !ok {
		return 1
	}
	intValue, ok := value.(bdf.TagInt)
	if !ok {
		return 1
	}
	*output = C.int32_t(int32(intValue))
	return 0
}

//export BDFNativeGetLong
func BDFNativeGetLong(data *C.uchar, length C.int, keyData *C.uchar, keyLength C.int, output *C.int64_t) (result C.int) {
	defer func() {
		if recover() != nil {
			result = -1
		}
	}()
	root, err := readRoot(data, length)
	if err != nil || keyLength < 0 || (keyLength > 0 && keyData == nil) {
		return -1
	}
	key := ""
	if keyLength > 0 {
		key = string(unsafe.Slice((*byte)(unsafe.Pointer(keyData)), int(keyLength)))
	}
	value, ok := root.Get(key)
	if !ok {
		return 1
	}
	longValue, ok := value.(bdf.TagLong)
	if !ok {
		return 1
	}
	*output = C.int64_t(int64(longValue))
	return 0
}

//export BDFNativeGetDouble
func BDFNativeGetDouble(data *C.uchar, length C.int, keyData *C.uchar, keyLength C.int, output *C.double) (result C.int) {
	defer func() {
		if recover() != nil {
			result = -1
		}
	}()
	root, err := readRoot(data, length)
	if err != nil || keyLength < 0 || (keyLength > 0 && keyData == nil) {
		return -1
	}
	key := ""
	if keyLength > 0 {
		key = string(unsafe.Slice((*byte)(unsafe.Pointer(keyData)), int(keyLength)))
	}
	value, ok := root.Get(key)
	if !ok {
		return 1
	}
	doubleValue, ok := value.(bdf.TagDouble)
	if !ok {
		return 1
	}
	*output = C.double(doubleValue)
	return 0
}

func main() {}
