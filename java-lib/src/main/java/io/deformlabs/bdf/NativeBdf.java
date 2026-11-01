package io.deformlabs.bdf;

/** Java JNI access to the Go BDF reader. Build libbdfjni.so before use. */
public final class NativeBdf {
    static {
        System.loadLibrary("bdfjni");
    }

    private NativeBdf() {}

    /** Returns whether the bytes contain a supported BDF file and root compound. */
    public static boolean isValid(byte[] data) {
        return nativeValidate(data);
    }

    public static int getInt(byte[] data, String key) {
        return nativeGetInt(data, key);
    }

    public static long getLong(byte[] data, String key) {
        return nativeGetLong(data, key);
    }

    public static double getDouble(byte[] data, String key) {
        return nativeGetDouble(data, key);
    }

    private static native boolean nativeValidate(byte[] data);
    private static native int nativeGetInt(byte[] data, String key);
    private static native long nativeGetLong(byte[] data, String key);
    private static native double nativeGetDouble(byte[] data, String key);
}
