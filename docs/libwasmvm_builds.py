

oss = ["Linux (glibc)", "Linux (musl)", "macOS", "Windows (mingw)"]
cpus = ["x86_64", "aarch64"]
build_types = ["shared", "static"]
# libc = ["glibc", "musl"]

ZERO_WIDTH_SPACE = "\u200B"
SUPPORTED = "✅" + ZERO_WIDTH_SPACE
UNSUPPORTED = "🚫" + ZERO_WIDTH_SPACE
UNKNOWN = "🤷" + ZERO_WIDTH_SPACE
UNDER_CONSTRUCTION = "🏗" + ZERO_WIDTH_SPACE

def wasmer22_supported(os, cpu, build_type):
    if os == "Windows (mingw)":
        if cpu == "x86_64" and build_type == "shared":
            return UNDER_CONSTRUCTION + "wasmvm.dll"
        else:
            return UNSUPPORTED
    if os == "macOS" and build_type == "static":
        return UNSUPPORTED
    if os == "macOS" and build_type == "shared":
        return SUPPORTED + "libwasmvm.dylib"
    if os == "Linux (musl)":
        if build_type == "static":
            if cpu == "x86_64":
                return SUPPORTED + "libwasmvm_muslc.x86_64.a"
            elif cpu == "aarch64":
                return SUPPORTED + "libwasmvm_muslc.aarch64.a"
        if build_type == "shared":
            return UNSUPPORTED
    if os == "Linux (glibc)":
        if build_type == "static":
            return UNSUPPORTED
        if build_type == "shared":
            if cpu == "x86_64":
                return SUPPORTED + "libwasmvm.x86_64.so"
            elif cpu == "aarch64":
                return SUPPORTED + "libwasmvm.aarch64.so"
    return UNKNOWN

def get_note(os, cpu, build_type):
    if os == "Windows (mingw)" and cpu == "x86_64" and build_type == "shared":
        return "See [#288]"
    if os == "Linux (glibc)" and cpu == "x86_64" and build_type == "static":
        return "Would link libwasmvm statically but glibc dynamically as static glibc linking is not recommended. Potentially interesting for Osmosis."
    if os == "Linux (musl)" and build_type == "shared":
        return "Possible but not needed"
    if os == "macOS" and build_type == "shared":
        return "Fat/universal library with multiple archs ([#294])"
    return ""

def get_links():
    return """
[#288]: https://github.com/CosmWasm/wasmvm/pull/288
[#294]: https://github.com/CosmWasm/wasmvm/pull/294
"""

print("<!-- AUTO GENERATED BY libwasmvm_builds.py START -->")
print("| OS family       | Arch    | Linking | Supported                     | Note    |")
print("| --------------- | ------- | ------- | ----------------------------- | ------- |")

for os in oss:
    for cpu in cpus:
        for build_type in build_types:
            s = wasmer22_supported(os, cpu, build_type)
            note = get_note(os, cpu, build_type)
            print(
                "| {:<15} | {:<7} | {:<7} | {:<29} | {} |".format(
                    os, cpu, build_type, s, note
                )
            )
print(get_links())
print("<!-- AUTO GENERATED BY libwasmvm_builds.py END -->")