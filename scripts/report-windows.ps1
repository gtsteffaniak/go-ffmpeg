param(
    [Parameter(Mandatory = $true)]
    [string]$Binary,

    [string]$OutputFile = "compatibility-report.txt"
)

$enc = [System.Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = $enc
$OutputEncoding = $enc

if (Test-Path $OutputFile) {
    Remove-Item $OutputFile -Force
}

& $Binary -color always 2>&1 | ForEach-Object {
    Write-Output $_
    [System.IO.File]::AppendAllText($OutputFile, $_ + [Environment]::NewLine, $enc)
}
