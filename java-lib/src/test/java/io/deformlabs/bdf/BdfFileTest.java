package io.deformlabs.bdf;

import java.nio.ByteBuffer;
import java.nio.ByteOrder;

public final class BdfFileTest {
    public static void main(String[] args) {
        byte[] bytes = fixture();
        BdfFile file = BdfFile.fromBytes(bytes);
        BdfFile.Compound root = file.root();

        assert root.size() == 2;
        assert root.getInt("answer").orElseThrow() == 42;
        assert root.getString("hello").orElseThrow().equals("hello");
        assert root.getInt("missing").isEmpty();
        assert root.keys().equals(java.util.List.of("answer", "hello"));
        assert NativeBdf.isValid(bytes);
        assert NativeBdf.getInt(bytes, "answer") == 42;

        bytes[0] = 0;
        assert file.root().getInt("answer").orElseThrow() == 42;
        assert !NativeBdf.isValid(bytes);
    }

    private static byte[] fixture() {
        ByteBuffer data = ByteBuffer.allocate(144).order(ByteOrder.LITTLE_ENDIAN);
        data.put(0, (byte) 'B');
        data.put(1, (byte) 'D');
        data.put(2, (byte) 'F');
        data.put(3, (byte) '1');
        data.putShort(4, (short) 1);
        data.putShort(6, (short) 0x31);
        data.putLong(8, 104);
        data.putLong(16, 64);
        data.putLong(24, 40);
        data.putLong(32, 144);
        data.putLong(40, 3);

        data.putInt(64, 2);
        data.putInt(68, 11);
        data.putInt(72, 0);
        data.putInt(76, 6);
        data.putInt(80, 6);
        data.putInt(84, 5);
        data.position(88);
        data.put("answerhello".getBytes(java.nio.charset.StandardCharsets.UTF_8));

        data.putInt(104, 2);
        data.putInt(108, 1);
        data.putInt(112, 0);
        data.put(116, (byte) 3);
        data.put(117, (byte) 1);
        data.putInt(120, 42);
        data.putInt(128, 1);
        data.put(132, (byte) 8);
        data.put(133, (byte) 1);
        data.putInt(136, 1);
        return data.array();
    }
}
