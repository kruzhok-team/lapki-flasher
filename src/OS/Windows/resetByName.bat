::source: https://github.com/p1ne/arduino-leonardo-uploader/blob/master/windows/upload.bat
@echo off
setlocal

for /f "tokens=1* delims==" %%I in ('wmic path win32_pnpentity get caption  /format:list ^| find "Arduino Micro"') do (
    call :resetCOM "%%~J"
)

:: end main batch
goto :EOF

:resetCOM <WMIC_output_line>
:: sets _COM#=line
setlocal
set "str=%~1"
set "num=%str:*(COM=%"
set "num=%num:)=%"
set port=COM%num%
echo %port%
mode %port%: BAUD=1200 parity=N data=8 stop=1
timeout 1

for /f "tokens=1* delims==" %%I in ('wmic path win32_pnpentity get caption  /format:list ^| find "Arduino Micro"') do (
    echo "%%~J"
)
::timeout 1
::AVRDUDE -p ATmega32u4 -c avr109 -P COM4 -b57600 -U flash:w:blink2.hex:i