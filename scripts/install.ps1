# Instala a ultima versao do FinFlow no Windows em %LOCALAPPDATA%\FinFlow.
#
#   irm https://raw.githubusercontent.com/danielharrisonlf/finflow/main/scripts/install.ps1 | iex
#
# Variaveis opcionais:
#   FINFLOW_INSTALL_DIR  pasta de destino (padrao: %LOCALAPPDATA%\FinFlow)
#   FINFLOW_VERSION      versao especifica, ex.: v0.2.0 (padrao: ultima)
#   FINFLOW_NO_PATH=1    nao adiciona a pasta ao PATH do usuario
#   FINFLOW_DOWNLOAD_URL base alternativa dos arquivos (espelho ou teste local)
# Mensagens sem acentos de proposito: o Windows PowerShell 5.1 le scripts sem BOM como ANSI.
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$repo = 'danielharrisonlf/finflow'
$installDir = if ($env:FINFLOW_INSTALL_DIR) { $env:FINFLOW_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'FinFlow' }
$baseUrl = if ($env:FINFLOW_DOWNLOAD_URL) {
  $env:FINFLOW_DOWNLOAD_URL.TrimEnd('/')
} elseif ($env:FINFLOW_VERSION) {
  "https://github.com/$repo/releases/download/$($env:FINFLOW_VERSION)"
} else {
  "https://github.com/$repo/releases/latest/download"
}

$cpu = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
$arch = if ($cpu -eq 'ARM64') { 'arm64' } else { 'amd64' }
$asset = "finflow-windows-$arch.zip"

$tmp = Join-Path ([IO.Path]::GetTempPath()) ("finflow-" + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
try {
  $zip = Join-Path $tmp $asset
  Write-Host "Baixando $asset..."
  Invoke-WebRequest -UseBasicParsing -Uri "$baseUrl/$asset" -OutFile $zip

  try {
    $sums = (Invoke-WebRequest -UseBasicParsing -Uri "$baseUrl/checksums.txt").Content
    if ($sums -is [byte[]]) { $sums = [Text.Encoding]::UTF8.GetString($sums) }
    $line = ($sums -split "`n") | Where-Object { $_ -match "\s$([regex]::Escape($asset))\s*$" } | Select-Object -First 1
    if ($line) {
      $expected = ($line -split '\s+')[0].ToLower()
      $actual = (Get-FileHash -Algorithm SHA256 -Path $zip).Hash.ToLower()
      if ($expected -ne $actual) { throw "checksum SHA-256 nao confere para $asset" }
    }
  } catch [System.Net.WebException] {
    Write-Host 'checksums.txt indisponivel; seguindo sem verificar o SHA-256.'
  }

  # Encerra uma instancia aberta para conseguir substituir o executavel.
  Get-Process -Name finflow -ErrorAction SilentlyContinue |
    Where-Object { $_.Path -and $_.Path.StartsWith($installDir, [StringComparison]::OrdinalIgnoreCase) } |
    Stop-Process -Force -ErrorAction SilentlyContinue

  New-Item -ItemType Directory -Force -Path $installDir | Out-Null
  Expand-Archive -Path $zip -DestinationPath $installDir -Force
} finally {
  Remove-Item -Recurse -Force -Path $tmp -ErrorAction SilentlyContinue
}

$exe = Join-Path $installDir 'finflow.exe'
Write-Host "FinFlow instalado em $exe ($(& $exe version))"

if (-not $env:FINFLOW_NO_PATH) {
  $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
  $parts = @($userPath -split ';' | Where-Object { $_ })
  if ($parts -notcontains $installDir) {
    [Environment]::SetEnvironmentVariable('Path', (($parts + $installDir) -join ';'), 'User')
    $env:Path = "$env:Path;$installDir"
    Write-Host "Pasta adicionada ao PATH do usuario (abra um novo terminal para usar 'finflow')."
  }
}

Write-Host ''
Write-Host 'Para comecar: finflow   (abre http://localhost:3333)'
Write-Host 'Com dados de exemplo: finflow serve --seed'
Write-Host "Ou de dois cliques em $exe"
