#!/usr/bin/env bash
source ./VERSION

name="sshw"
input="./cmd/sshw"

go=go

if [[ "$1" != "" ]];then
    version=$1
fi

output="out/"

OnlyBuild() {
    goarm=$4
    if [[ "$4" = "" ]];then
        goarm=7
    fi

    echo "Building $1..."
    export GOOS=$2 GOARCH=$3 GO386=sse2 CGO_ENABLED=0 GOARM=$4
    if [[ $2 = "windows" ]];then
        $go build -ldflags "-X main.Version=$version -s -w" -o "$output/$1/$name.exe" $input
    else
        $go build -ldflags "-X main.Version=$version -s -w" -o "$output/$1/$name" $input
    fi
}

Build() {
    OnlyBuild $*

    Pack $1
}

# zip 打包
Pack() {
    cp README.md "$output/$1"

    cd $output
    zip -q -r "$1.zip" "$1"

    # 删除
    rm -rf "$1"

    cd ..
}

InstallDarwin() {
    n=$name-$version"-darwin-osx-amd64"
    OnlyBuild ${n} darwin amd64
    mv $output/${n}/$name $(go env GOPATH)/bin
}

BuildAll() {
    # OS X / macOS
    Build $name-$version"-darwin-osx-amd64" darwin amd64
    # Windows
#    Build $name-$version"-windows-x86" windows 386
#    Build $name-$version"-windows-x64" windows amd64
    # Linux
    Build $name-$version"-linux-amd64" linux amd64
}
