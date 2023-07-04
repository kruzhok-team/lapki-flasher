::source: https://github.com/p1ne/arduino-leonardo-uploader/blob/master/windows/upload.bat
@echo off
setlocal

for /f "tokens=1* delims==" %%I in ('wmic path win32_pnpentity get caption  /format:list ^| find "Arduino Micro"') do (
    call :resetCOM "%%~J"
    echo %%~J
)

:continue

:: wmic /format:list strips trailing spaces (at least for path win32_pnpentity)
for /f "tokens=1* delims==" %%I in ('wmic path win32_pnpentity get caption  /format:list ^| find "Arduino Micro bootloader"') do (
    call :setCOM "%%~J"
    echo %%~J
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
timeout 2
goto :continue

:setCOM <WMIC_output_line>
:: sets _COM#=line
setlocal
set "str=%~1"
set "num=%str:*(COM=%"
set "num=%num:)=%"
set port=COM%num%
echo %port%
goto :flash

:flash
AVRDUDE -p ATmega32u4 -c avr109 -P %port% -b57600 -U flash:w:blink2.hex:i