::source: https://github.com/p1ne/arduino-leonardo-uploader/blob/master/windows/upload.bat
@echo off
setlocal

:: wmic /format:list strips trailing spaces (at least for path win32_pnpentity)
for /f "tokens=1* delims==" %%I in ('wmic path win32_pnpentity get caption  /format:list ^| find "Arduino Micro"') do (
    call :setCOM "%%~J"
)

:: end main batch
goto :EOF

:setCOM <WMIC_output_line>
:: sets _COM#=line
setlocal
set "str=%~1"
set "num=%str:*(COM=%"
set "num=%num:)=%"
set port=COM%num%
echo %port%