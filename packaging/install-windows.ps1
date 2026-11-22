[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$Binary
)

$ErrorActionPreference = 'Stop'
$source = Get-Item -LiteralPath $Binary
if ($source.PSIsContainer) {
    throw 'Binary must be the path to the built executable.'
}

$localAppData = [Environment]::GetFolderPath([Environment+SpecialFolder]::LocalApplicationData)
$installDir = Join-Path $localAppData 'Programs\object-N-A'
$executable = Join-Path $installDir 'object-n-a.exe'
New-Item -ItemType Directory -Path $installDir -Force | Out-Null
Copy-Item -LiteralPath $source.FullName -Destination $executable -Force

$programs = [Environment]::GetFolderPath([Environment+SpecialFolder]::Programs)
New-Item -ItemType Directory -Path $programs -Force | Out-Null
$shortcutPath = Join-Path $programs 'object-N-A.lnk'
$shell = New-Object -ComObject WScript.Shell
$shortcut = $shell.CreateShortcut($shortcutPath)
$shortcut.TargetPath = $executable
$shortcut.IconLocation = "$executable,0"
$shortcut.WorkingDirectory = $installDir
$shortcut.Description = 'object-N/A Markdown Workspace'
$shortcut.Save()

Write-Output $shortcutPath
