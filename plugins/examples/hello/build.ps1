# Builds plugin.wasm (Go -> WebAssembly reactor) and packs dist\<id>-<version>.zip, signed with
# dev.key (created on the first run). Trust dev.pub under Plugins -> Settings -> Trusted publisher
# keys, then upload the zip under Plugins -> Installed.
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
$env:GOOS = 'wasip1'
$env:GOARCH = 'wasm'
try {
  go build -buildmode=c-shared -o plugin.wasm .
} finally {
  Remove-Item Env:GOOS, Env:GOARCH
}
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
if (-not (Test-Path dev.key)) {
  go run ../../tools/loungeplugin keygen -out dev
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
go run ../../tools/loungeplugin pack -dir . -key dev.key -out dist
exit $LASTEXITCODE
