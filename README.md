# tmpcontrol: a lot cheaper than a kegerator

tmpcontrol runs on a raspberry pi with a DS18B20 temperature sensor and can control a heater/fridge with a Kasa smart plug.

## Features

- JSON configuration-driven
- Standalone mode or push config from the web
- Free use of our server (as long as we can maintain it😀️), or host your own
- Temperature configuration can be scheduled, for example, for a fermentation temperature schedule
- If you have a heating element, you can configure the mash water to be preheated by the morning
- If you would like to receive text message notifications, you can Venmo me a few bucks to pay Twilio

## Usage

```
-kasa-path /home/pi/.local/bin/kasa
-config-server-root-url 
-local-config-path pi-config.json
-client-identifier johns-basement
-config-fetch-interval 60
```

## Setup

1. Fetch the code base

   `code goes here`
   
   or download the binary directly to the raspberry pi
   
   `wget ...`
2. (optional if you already have the binary) Cross-compile the binary for your Pi
   
   ```
   cd cmd/tmpcontrol
   GOOS=linux GOARCH=arm GOARM=7 go build -o ../../bin/tmpcontrol
   ```
3. Arrange to send the binary to the Pi via sftp or by copying it to the SD card
4. Test that the program runs
   `tmpcontrol`
5. Create a bash script to run tmpcontrol. This allows you to do some extra setup. `nano start-temperature-control.sh`
   ```
   #!/bin/bash

   # check what pid(s) match this script's name. If there is just one (ours), then we'll launch the controls
   echo Our PID: $$
   pids=$(/usr/bin/pgrep -f "/bin/bash /home/pi/start-temperature-control.sh")
   echo All PIDs with this script name: $pids
   if [ "$pids" = "$$" ]; then
     export ADMIN_NOTIFY_KEY="xxxxxxxxxxxx"
     export ADMIN_NOTIFY_NUMBER="+112355505678"
     /home/pi/tmpcontrol -local-config-path pi-config.json -kasa-path /home/pi/.local/bin/kasa >> /home/pi/temperature-control.out 2>&1
   else
     echo 'There is an instance running already.'
   fi
   ```
   
   `chmod +x start-temperature-control.sh`
6. Schedule the bash script to run every 5 minutes in case it dies. Run `crontab -e` and add the line
   ```
   */5 * * * * /home/pi/start-temperature-control.sh
   ```
   You may also want to add a line to reboot the Pi daily in case memory is leaking for whatever reason (note the `sudo` to edit root's crontab):
   ```
   sudo crontab -e
   # add the following line to crontab to reboot at 6 pm daily
   0 18 * * * /usr/sbin/reboot
   ```

7. Let tmpcontrol start on its own and monitor the output:
   
   `tail -f temperature-control.out`

## Configuration examples

### Prep mash water for when you wake up in the morning

### Keep kegs ready to serve, 33°F

### Fermentation with cold crash

### Lager fermentation with diacetyl rest

## Pending work

- [ ] Provide Celsius support
- [ ] Allow for email notifications sent from server
- [ ] Make some of the notification intervals configurable by command line

## FAQ

### How do I configure the temperature sensor and find its PATH?

