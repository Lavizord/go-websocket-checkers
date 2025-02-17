@echo off
setlocal enableextensions enabledelayedexpansion

:: Start Memurai
start "" "C:\Program Files\Memurai\memurai.exe"

:: Start Go programs and store their PIDs
start "" cmd /c "go run C:\Projetos\golang\go-websocket-prototype\main.go" & echo !PID! > main.pid
start "" cmd /c "go run C:\Projetos\golang\go-websocket-prototype\pstatusworker\main.go" & echo !PID! > pstatusworker.pid
start "" cmd /c "go run C:\Projetos\golang\go-websocket-prototype\roomworker\main.go" & echo !PID! > roomworker.pid
start "" cmd /c "go run C:\Projetos\golang\go-websocket-prototype\broadcastworker\main.go" & echo !PID! > broadcastworker.pid

echo All services started.
echo Press Ctrl+C to stop all services.
pause

:: Cleanup - Kill processes on exit
echo Stopping services...
taskkill /F /IM memurai.exe
for %%f in (main.pid pstatusworker.pid roomworker.pid broadcastworker.pid) do (
    if exist %%f (
        set /p pid=<%%f
        taskkill /F /PID !pid!
        del %%f
    )
)
exit /b
