//go:build jni

#include <jni.h>
#include <stdint.h>
#include "_cgo_export.h"

static void throw_argument(JNIEnv *env, const char *message) {
    jclass exception = (*env)->FindClass(env, "java/lang/IllegalArgumentException");
    if (exception != NULL) {
        (*env)->ThrowNew(env, exception, message);
    }
}

static int input_bytes(JNIEnv *env, jbyteArray input, jbyte **bytes, jsize *length) {
    if (input == NULL) {
        throw_argument(env, "BDF bytes must not be null");
        return 0;
    }
    *length = (*env)->GetArrayLength(env, input);
    *bytes = (*env)->GetByteArrayElements(env, input, NULL);
    return *bytes != NULL;
}

JNIEXPORT jboolean JNICALL
Java_io_deformlabs_bdf_NativeBdf_nativeValidate(JNIEnv *env, jclass type, jbyteArray input) {
    jbyte *bytes;
    jsize length;
    if (!input_bytes(env, input, &bytes, &length)) {
        return JNI_FALSE;
    }
    int status = BDFNativeValidate((unsigned char *)bytes, length);
    (*env)->ReleaseByteArrayElements(env, input, bytes, JNI_ABORT);
    return status == 0 ? JNI_TRUE : JNI_FALSE;
}

static int get_key(JNIEnv *env, jstring key, jbyteArray *key_bytes, jbyte **bytes, jsize *length) {
    if (key == NULL) {
        throw_argument(env, "BDF key must not be null");
        return 0;
    }
    jclass string_class = (*env)->FindClass(env, "java/lang/String");
    jmethodID get_bytes = (*env)->GetMethodID(env, string_class, "getBytes", "(Ljava/lang/String;)[B");
    jstring charset = (*env)->NewStringUTF(env, "UTF-8");
    *key_bytes = (jbyteArray)(*env)->CallObjectMethod(env, key, get_bytes, charset);
    (*env)->DeleteLocalRef(env, charset);
    if ((*env)->ExceptionCheck(env) || *key_bytes == NULL) {
        return 0;
    }
    *length = (*env)->GetArrayLength(env, *key_bytes);
    *bytes = (*env)->GetByteArrayElements(env, *key_bytes, NULL);
    return *bytes != NULL;
}

static int prepare_lookup(JNIEnv *env, jbyteArray input, jstring key,
                          jbyte **data, jsize *data_length,
                          jbyteArray *key_bytes, jbyte **key_data, jsize *key_length) {
    if (!input_bytes(env, input, data, data_length)) {
        return 0;
    }
    if (!get_key(env, key, key_bytes, key_data, key_length)) {
        (*env)->ReleaseByteArrayElements(env, input, *data, JNI_ABORT);
        return 0;
    }
    return 1;
}

static void finish_lookup(JNIEnv *env, jbyteArray input, jbyte *data,
                          jbyteArray key_bytes, jbyte *key_data) {
    (*env)->ReleaseByteArrayElements(env, input, data, JNI_ABORT);
    (*env)->ReleaseByteArrayElements(env, key_bytes, key_data, JNI_ABORT);
    (*env)->DeleteLocalRef(env, key_bytes);
}

static int check_status(JNIEnv *env, int status) {
    if (status == 0) {
        return 1;
    }
    throw_argument(env, status == 1
        ? "BDF key is missing or has a different tag type"
        : "invalid or unsupported BDF data");
    return 0;
}

JNIEXPORT jint JNICALL
Java_io_deformlabs_bdf_NativeBdf_nativeGetInt(JNIEnv *env, jclass type,
                                               jbyteArray input, jstring key) {
    jbyte *data = NULL, *key_data = NULL;
    jsize data_length, key_length;
    jbyteArray key_bytes;
    if (!prepare_lookup(env, input, key, &data, &data_length, &key_bytes, &key_data, &key_length)) {
        return 0;
    }
    int32_t value = 0;
    int status = BDFNativeGetInt((unsigned char *)data, data_length,
                                 (unsigned char *)key_data, key_length, &value);
    finish_lookup(env, input, data, key_bytes, key_data);
    return check_status(env, status) ? (jint)value : 0;
}

JNIEXPORT jlong JNICALL
Java_io_deformlabs_bdf_NativeBdf_nativeGetLong(JNIEnv *env, jclass type,
                                                jbyteArray input, jstring key) {
    jbyte *data = NULL, *key_data = NULL;
    jsize data_length, key_length;
    jbyteArray key_bytes;
    if (!prepare_lookup(env, input, key, &data, &data_length, &key_bytes, &key_data, &key_length)) {
        return 0;
    }
    int64_t value = 0;
    int status = BDFNativeGetLong((unsigned char *)data, data_length,
                                  (unsigned char *)key_data, key_length, &value);
    finish_lookup(env, input, data, key_bytes, key_data);
    return check_status(env, status) ? (jlong)value : 0;
}

JNIEXPORT jdouble JNICALL
Java_io_deformlabs_bdf_NativeBdf_nativeGetDouble(JNIEnv *env, jclass type,
                                                  jbyteArray input, jstring key) {
    jbyte *data = NULL, *key_data = NULL;
    jsize data_length, key_length;
    jbyteArray key_bytes;
    if (!prepare_lookup(env, input, key, &data, &data_length, &key_bytes, &key_data, &key_length)) {
        return 0;
    }
    double value = 0;
    int status = BDFNativeGetDouble((unsigned char *)data, data_length,
                                    (unsigned char *)key_data, key_length, &value);
    finish_lookup(env, input, data, key_bytes, key_data);
    return check_status(env, status) ? (jdouble)value : 0;
}
