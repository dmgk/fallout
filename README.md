## fallout

Download and search FreeBSD build cluster fallout logs.

#### Installation

    go install github.com/dmgk/fallout@latest

#### Usage

```
usage: fallout [-hV] [-M mode] [-G colors] command [options]

Download and search fallout logs.

Options:
  -h              show help and exit
  -V              show version and exit
  -M mode         color mode [auto|never|always] (default: auto)
  -G colors       set colors (default: "BCDA")
                  the order is query,match,path,separator; see ls(1) for color codes

Commands (pass -h for command help):
  fetch           download fallout logs
  grep            search fallout logs
  clean           clean log cache
  stats           show cache statistics
```

##### Fetching failure logs:

```
usage: fallout fetch [-h] [-D days] [-A date] [-N count] [-b builder[,builder]] [-c category[,category]] [-o origin[,origin]] [-n name[,name]]

Download and cache fallout logs.

Options:
  -h              show help and exit
  -D days         download logs for the last days (default: 7)
  -A date         download only logs after this date, in RFC-3339 format (default: 2022-07-07)
  -N count        download only recent count logs
  -b builder,...  download only logs from these builders
  -c category,... download only logs for these categories
  -o origin,...   download only logs for these origins
  -n name,...     download only logs for these port names
```

###### Searching:

```
usage: fallout grep [-hFOl] [-A count] [-B count] [-C count] [-b builder[,builder]] [-c category[,category]] [-o origin[,origin]] [-n name[,name]] [-s since] [-e before] [-j jobs] query [query ...]

Search cached fallout logs.

Options:
  -h              show help and exit
  -F              interpret query as a plain text, not regular expression
  -O              multiple queries are OR-ed (default: AND-ed)
  -l              print only matching log filenames
  -A count        show count lines of context after match
  -B count        show count lines of context before match
  -C count        show count lines of context around match
  -b builder,...  limit search only to these builders
  -c category,... limit search only to these categories
  -o origin,...   limit search only to these origins
  -n name,...     limit search only to these port names
  -s since        list only failures since this date or date-time, in RFC-3339 format
  -e before       list only failures before this date or date-time, in RFC-3339 format
  -j jobs         number of parallel jobs, -j1 outputs sorted results (default: 8)
```

##### Cleaning the cache:

```
usage: fallout clean [-hx] [-D days] [-A date]

Clean log cache.

Options:
  -h          show help and exit
  -x          remove all cached data
  -D days     remove logs that are more than days old (default: 30)
  -A date     remove logs that are older than date, in RFC-3339 format (default: 2022-06-14)
```

#### Examples:

Run `fallout fetch` to download recent logs and then:

##### Search logs of broken USES=go ports across all builders:

```sh
$ portgrep -u go -1 | ./fallout grep -C2 "\.go:\d+:\d+:"
/home/dg/.cache/fallout/main-i386-default/security/honeytrap/2022-07-14T15:50:32.log:
github.com/honeytrap/honeytrap/services/telnet
# github.com/honeytrap/honeytrap/services/docker
services/docker/docker@@.go:405:23:@@ cannot use 16348065792 (untyped int constant) as int value in map literal (overflows)
github.com/honeytrap/honeytrap/pushers/raven
github.com/honeytrap/honeytrap/pushers/pulsar
/home/dg/.cache/fallout/130arm64-quarterly/sysutils/aptly/2022-07-07T23:53:07.log:
google.golang.org/protobuf/reflect/protoreflect
# golang.org/x/sys/unix
vendor/golang.org/x/sys/unix/syscall_freebsd_arm64.go:60:1: syntax error: non-declaration statement outside function body
vendor/golang.org/x/sys/unix/zerrors_freebsd_arm64.go:1798:3: misplaced compiler directive
vendor/golang.org/x/sys/unix/zerrors_freebsd_arm64.go:1804:1: syntax error: non-declaration statement outside function body
...
```

##### List logs for arm64 failures:

```sh
$ fallout grep -b arm64 -l
/home/user/.cache/fallout/main-arm64-default/www/mod_gnutls/2022-07-06T02:23:34.log
/home/user/.cache/fallout/main-arm64-default/textproc/nunnimcax/2022-07-06T03:10:08.log
/home/user/.cache/fallout/main-arm64-default/www/trac-devel/2022-07-05T20:17:35.log
/home/user/.cache/fallout/main-arm64-default/x11-toolkits/wxgtk28-common/2022-07-04T17:08:30.log
/home/user/.cache/fallout/main-arm64-default/x11-wm/afterstep-stable/2022-07-07T23:28:41.log
/home/user/.cache/fallout/main-arm64-default/x11-toolkits/gtkd/2022-07-05T05:59:07.log
...
```

##### Search by an arbitrary regex:

```sh
$ fallout grep -C1 -c devel -x "\sundefined\s"
/home/user/.cache/fallout/main-armv7-default/devel/cvs-devel/2022-07-11T22:14:29.log:
                                                     ^~~~~~~~~~~~~~~~~~~~
mktime.c:211:56: warning: shifting a negative signed value is undefined [-Wshift-negative-value]
      if ((t1 < *t) == (TYPE_SIGNED (time_t) ? d < 0 : TIME_T_MAX / 2 < d))
/home/user/.cache/fallout/main-riscv64-default/devel/arm-none-eabi-gcc492/2022-07-05T18:44:32.log:
c++: warning: treating 'c' input as 'c++' when in C++ mode, this behavior is deprecated [-Wdeprecated]
/wrkdirs/usr/ports/devel/arm-none-eabi-gcc492/work/gcc-4.9.2/gcc/lto/lto.c:1831:4: warning: performing pointer subtraction with a null pointer may have undefined behavior [-Wnull-pointer-subtraction]
        = XOBNEWVAR (&tree_scc_hash_obstack, tree_scc, sizeof (tree_scc));
/home/user/.cache/fallout/main-riscv64-default/devel/arm-none-eabi-gcc492/2022-07-12T10:20:07.log:
c++: warning: treating 'c' input as 'c++' when in C++ mode, this behavior is deprecated [-Wdeprecated]
/wrkdirs/usr/ports/devel/arm-none-eabi-gcc492/work/gcc-4.9.2/gcc/lto/lto.c:1831:4: warning: performing pointer subtraction with a null pointer may have undefined behavior [-Wnull-pointer-subtraction]
        = XOBNEWVAR (&tree_scc_hash_obstack, tree_scc, sizeof (tree_scc));
```
