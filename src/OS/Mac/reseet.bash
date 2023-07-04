# https://stackoverflow.com/questions/43016993/how-can-i-force-an-arduino-leonardo-to-reset-with-avrdude
# Find the Arduino port
ARDUINO_UPLOAD_PORT="$(find /dev/cu.usbmodem* | head -n 1)"

# Reset the Arduino
stty -f "${ARDUINO_UPLOAD_PORT}" 1200

# Wait for it...
while :; do
  sleep 0.5
  [ -c "${ARDUINO_UPLOAD_PORT}" ] && break
done

# ...upload!
avrdude "${OPTIONS[@]}"