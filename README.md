## fallout

Download and search FreeBSD build cluster fallout logs.

#### Installation

    go install github.com/dmgk/fallout@latest

#### Usage

```
usage: fallout [-hv] command [options]

Download and search fallout logs.

Options:
  -h          show help and exit
  -V          show version and exit

Commands (pass -h for command help):
  fetch       download fallout logs
  grep        search fallout logs
  clean       clean log cache
```

```
usage: fallout fetch [-h] [-d days] [-a date] [-n count]

Download and cache fallout logs.

Options:
  -h          show help and exit
  -d days     download logs for the last days (default: 7)
  -a date     download only logs after this date, in RFC-3339 format (default: 2022-07-05)
  -n count    download only recent count logs
```

```
usage: fallout grep [-hx] [-A count] [-B count] [-C count] [-b builder[,builder]] [-c category[,category]] [-o origin[,origin]] [-n name[,name]] query [query ...]

Search cached fallout logs.

Options:
  -h          show help and exit
  -A count    show count lines of context after match
  -B count    show count lines of context before match
  -C count    show count lines of context around match
  -b builder  limit search only to this builder
  -c category limit search only to this category
  -o origin   limit search only to this origin
  -n name     limit search only to this port name
  -x          treat query as a regular expression
  -O          multiple queries are OR-ed (default: AND-ed)
  -l          print only matching log filenames
  -M          color mode [auto|never|always] (default: auto)
  -G colors   set colors (default: "BCDA")
              the order is query,match,path,separator; see ls(1) for color codes
```

```
usage: fallout clean [-hx] [-d days] [-a date]

Clean log cache.

Options:
  -h          show help and exit
  -d days     remove logs that are more than days old (default: 30)
  -a date     remove logs that are older than date, in RFC-3339 format (default: 2022-06-12)
  -x          remove all cached data
```

#### Examples:

Run `fallout fetch` to download recent logs and then:

List logs for broken USES=go ports across all builders:

```sh
$ fallout grep -o "$(portgrep -u go -1)" -l
/home/user/.cache/fallout/main-i386-default/security/vaultwarden/2022-07-05T12:51:26.log
/home/user/.cache/fallout/main-i386-default/sysutils/minikube/2022-07-05T05:26:17.log
/home/user/.cache/fallout/main-i386-default/www/minio/2022-07-09T23:14:53.log
/home/user/.cache/fallout/main-amd64-default/databases/mongodb36-tools/2022-07-12T07:04:35.log
/home/user/.cache/fallout/130i386-quarterly/security/vaultwarden/2022-07-10T14:02:29.log
/home/user/.cache/fallout/130i386-quarterly/www/minio/2022-07-07T01:49:12.log
...
```

List logs for arm64 failures:

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

Search by an arbitrary regex:

```sh
$ fallout grep -C1 -o devel -x "\sundefined\s"
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
