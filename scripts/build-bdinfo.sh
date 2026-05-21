#!/bin/sh
# BDInfo 独立构建脚本
# 在部署时自动构建 BDInfo，无需将其打包到主镜像中
set -eux

BDINFO_REPO="${BDINFO_REPO:-https://github.com/mirrorb/BDInfoCLI.git}"
BDINFO_REF="${BDINFO_REF:-e43d585775c902beb4d1206a9853c4b3dcec8aa2}"
BDINFO_CSPROJ="${BDINFO_CSPROJ:-BDInfo/BDInfo.csproj}"
ALPINE_MIRROR="${ALPINE_MIRROR:-mirrors.aliyun.com}"
GITHUB_MIRROR="${GITHUB_MIRROR:-https://githubfast.com}"

# 检测架构
case "$(uname -m)" in
    x86_64|amd64) RID="linux-musl-x64" ;;
    aarch64|arm64) RID="linux-musl-arm64" ;;
    *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

apk add --no-cache git ca-certificates

# 使用 GitHub 镜像加速
MIRROR_REPO=$(echo "$BDINFO_REPO" | sed "s|https://github.com/|${GITHUB_MIRROR}/|")
git init /src/bdinfo
cd /src/bdinfo
git remote add origin "$MIRROR_REPO"
git fetch --depth 1 origin "$BDINFO_REF" || {
    echo "Mirror fetch failed, trying direct..." >&2
    git remote set-url origin "$BDINFO_REPO"
    git fetch --depth 1 origin "$BDINFO_REF"
}
git checkout FETCH_HEAD

dotnet restore "$BDINFO_CSPROJ"
dotnet publish "$BDINFO_CSPROJ" -c Release -r "$RID" --self-contained true \
    -p:PublishSingleFile=true \
    -p:EnableCompressionInSingleFile=true \
    -p:DebugType=None \
    -p:DebugSymbols=false \
    -o /out/bdinfo

# 查找并重命名可执行文件
exe=""
for f in /out/bdinfo/*; do
    [ -f "$f" ] || continue
    [ -x "$f" ] || continue
    case "${f##*.}" in
        dll|json|pdb) continue ;;
    esac
    exe="$f"
    break
done

if [ -n "$exe" ]; then
    if [ "$exe" != "/out/bdinfo/BDInfo" ]; then
        mv "$exe" /out/bdinfo/BDInfo
    fi
else
    echo "BDInfo executable not found" >&2
    exit 1
fi

chmod +x /out/bdinfo/BDInfo
find /out/bdinfo -type f \( -name '*.pdb' -o -name '*.xml' -o -name '*.dbg' \) -delete

# 复制到共享卷
cp /out/bdinfo/BDInfo /shared/BDInfo
chmod +x /shared/BDInfo
echo "BDInfo built successfully: /shared/BDInfo"