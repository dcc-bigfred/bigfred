loco
====

CLI utility to work with DCC equpied locomotives and wagons.

BigFred SPA (`web/`)
--------------------

The React frontend rasterizes function icons (`web/src/icons/*.svg` →
`web/src/icons/png/*.png`, gitignored) during `vite` / `vite build` via a
Vite plugin. Requires **`rsvg-convert`** from librsvg:

```bash
# Arch / Manjaro
pacman -S librsvg

# Debian / Ubuntu
apt install librsvg2-bin
```

You can also run rasterization manually:

```bash
make web-icons
```

Setup
-----

1. Create a global configuration file

```bash
nano ~/.loco.yaml
```

2. Adjust & put configuration details

```yaml
server:
    type: "z21"
    address: "192.168.0.111"
    port: "21105"
```

Sending function commands (Lenz LAN)
------------------------------------

You can toggle a locomotive function (e.g. F0-F28) directly over a Lenz LAN
TCP connection (default port 5550) using the `fn send` subcommand:

```bash
# Toggle F3 on locomotive 3 (turn on)
$ loco fn send -l 3 -f 3 -o

# Turn F3 off
$ loco fn send -l 3 -f 3 --on=false

# Specify a different host (port is fixed to 5550)
$ loco fn send -H 192.168.0.50 -l 17 -f 0 -o
```

Flags:

* `-H, --host`  Lenz command station host (port 5550 is fixed)
* `-l, --loco`  Locomotive DCC address
* `-f, --fn`    Function number to toggle
* `-o, --on`    Turn the function on (use `--on=false` to turn off)
* `-t, --timeout` Connection timeout in seconds
* `-v, --debug` Enable debug logging (shows the raw frame bytes)

The frame sent follows the XpressNet LAN_X_SET_LOCO_FUNCTION structure:
`E4 F8 <AdrLSB> <AdrMSB> <GroupType> <GroupState> <XOR>`.


Working with CV's
-----------------

### Retrieving multiple CVs

```bash
$ loco cv get cv1,cv2
cv1=17
cv2=2
```

#### Advanced syntax

```bash
$ loco cv get cv52-53=0, cv1, cv5
cv1=17
cv5=255
cv52=0
cv53=0
```

### Specyfing a range of CVs

```bash
$ loco cv get cv1-cv255
cv1=17
cv2=2
# ...
cv255=5
```

### Retrieving a single CV

```bash
$ loco cv get cv1
17
```

### Specyfing a track type

```bash
# -t: track type, could be pom or prog
# -l: locomotive address
$ loco cv get cv2 -t pom -l 17
5

# when the "-l" is not specified the programming track is automatically chosen
$ loco cv get cv2
5
```

### Increasing verbosity

Logging messages are sent via stderr, results are in stdout.

```bash
$ loco cv get -v cv52-53=0, cv1, cv5
DEBU[0000] Reading configuration files                  
DEBU[0000] Initializing command station                 
DEBU[0000] z21.sendAndAwait([]byte = [1001 0 1000000 0 100011 10001 0 0 110010]) 
DEBU[0001] Marking programmng track as to be powered off 
cv1=17
DEBU[0001] z21.sendAndAwait([]byte = [1001 0 1000000 0 100011 10001 0 100 110110]) 
DEBU[0002] Marking programmng track as to be powered off 
cv5=255
DEBU[0002] z21.sendAndAwait([]byte = [1001 0 1000000 0 100011 10001 0 110011 1]) 
DEBU[0003] Marking programmng track as to be powered off 
cv52=0
DEBU[0003] z21.sendAndAwait([]byte = [1001 0 1000000 0 100011 10001 0 110100 110]) 
DEBU[0004] Marking programmng track as to be powered off 
cv53=0
DEBU[0004] Restoring power on programming track
```

### Setting a timeout

> Notice: Timeout = 0 does not mean no timeout at all, it means 0 seconds, so all commands would fail immediately

```bash
$ loco cv get cv52-53=0, cv1, cv5 --timeout 0
cv1=ERROR
ERRO[0000] cannot read CV: no response or unrecognized response 
cv5=ERROR
ERRO[0001] cannot read CV: no response or unrecognized response 
cv52=ERROR
ERRO[0001] cannot read CV: no response or unrecognized response 
cv53=ERROR
ERRO[0002] cannot read CV: no response or unrecognized response 
Error: cannot read CV: no response or unrecognized response
Usage:
  loco cv get [flags]

Flags:
  -v, --debug            Increase verbosity to the debug level
  -h, --help             help for get
  -l, --loco uint8       Use locomotive under specific address
      --timeout uint16   Connection timeout (default 10)
  -t, --track string     Track type: 'pom' for programming on main, 'prog' for programming track, or empty for automatic selection
      --verify           Verify the value after writting
```


### Backup & Restore CV

CLI command gives an advantage over UI interfaces with a possibility of scripting. Backup & Restore is a natural use case of using CLI. You can experiment with various locomotive settings having multiple CV settings and quickly move between them by massivly dumping and loading the values.

#### Get a list of selected CVs defined in a file

```bash
$ cat ./examples/cv-read.example.txt
cv1
cv2
cv3-cv4

$ cat ./examples/cv-read.example.txt | loco cv get -- -
cv1=17
cv2=2
cv3=34
cv4=25
```

#### Complete backup & restore workflow

```bash
# Receive CV1, CV2, CV3, CV4 and save to backup-cv.txt file
$ loco cv get cv2-cv4 > backup-cv.txt

# Then restore it to a locomotive from a backup-cv.txt file anytime
$ cat backup-cv.txt | loco cv set -v -- -
```

Toggling functions
------------------

```bash
# list functions that are in "on" state
$ loco fn list -l 3
No active functions

# toggle F0 and F5 to "on"
$ loco fn set 0 -l 3
$ loco fn set 5 -l 3

# list functions, there should be two listed that we turned "on"
$ loco fn list -l 3
F0 = On
F5 = On

# now we turn off F5
$ loco fn set 5 -l 3 --off

# so the F0 should be listed only
$ loco fn list -l 3
F0 = On
```
