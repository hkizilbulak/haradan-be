Get-Content .env | ForEach-Object {
    $line = $_.Trim()
    if ($line -and -not $line.StartsWith('#')) {
        $parts = $line.Split('=', 2)
        if ($parts.Count -eq 2) {
            $key = $parts[0].Trim()
            $value = $parts[1].Trim()
            [System.Environment]::SetEnvironmentVariable($key, $value, 'Process')
        }
    }
}

if ($args.Count -eq 0) {
    go run ./cmd/api
} else {
    $cmd = $args -join ' '
    cmd.exe /c $cmd
}
