# BDF Java reader

This Java 17+ library provides read-only, random-access access to BDF v1 files.
Compound values remain views over the encoded data rather than being decoded
into a tree. It supports uncompressed and gzip-compressed files; Zstandard
requires a decoder dependency and is currently unsupported.

## Build the Java library

Run:

```sh
./build.sh
```

This creates the dependency-free Java archive at `build/bdf-java.jar`. Use it
from another Java project by adding the jar to the classpath:

```sh
javac -cp java-lib/build/bdf-java.jar Example.java
java -cp java-lib/build/bdf-java.jar:. Example
```

`BdfFile` works without native libraries. `NativeBdf` is an optional JNI
interface and requires the separate Go-built `libbdfjni.so` described below.

## Go-backed JNI library

On Linux with Go (cgo enabled) and a JDK installed, run the Go project's build
script from `application/`:

```sh
./scripts/build.sh
```

This writes `java-lib/build/libbdfjni.so`. Add that directory to
`java.library.path` to use `NativeBdf`, which delegates validation and
root-compound `int`, `long`, and `double` lookups to the Go BDF implementation:

```java
byte[] data = java.nio.file.Files.readAllBytes(Path.of("level.bdf"));
int dataVersion = NativeBdf.getInt(data, "DataVersion");
```

`NativeBdf` throws `IllegalArgumentException` for malformed/unsupported input,
or if a key is absent or has a different tag type. The JNI library is Linux-only
in this build script; compile a separate native library for each target OS and
architecture.

Compile and run the standalone smoke test with the JDK after building the JNI
library:

```sh
mkdir -p build/test-classes
javac --release 17 -d build/test-classes \
  $(find src/main/java src/test/java -name '*.java')
java -ea -Djava.library.path=build -cp build/test-classes \
  io.deformlabs.bdf.BdfFileTest
```

Example:

```java
import io.deformlabs.bdf.BdfFile;
import java.nio.file.Path;

BdfFile.Compound root = BdfFile.open(Path.of("level.bdf")).root();
int dataVersion = root.getInt("DataVersion").orElseThrow();
```

Typed primitive getters return an empty `Optional` when the key is absent or
has another tag type. Nested compounds and lists are also exposed as views.
