# roundtrip
A commandline tool to check forward and reverse DNS for a CSV of hostnames or IP addresses

```
Usage: roundtrip [--out=out.csv] [file.csv]
--column string     Look for addresses or hostnames in this column (default "1")
--discards string   Write bad input lines to this csv file
-h, --help              Print this help and exit
-o, --out string        Send output CSV to this file
--version           Print version information and exit
```

`roundtrip` takes a single CSV file on the commandline. The first line of that CSV file should contain column names.

By default it expects the first column to contain an IP address or a hostnames. You can pass a column name or number
with the --column flag to use another column for input. `roundtrip` will check forward and reverse
DNS for each record, and add three new columns to the end of each line of the CSV.

The first added column contains the IP addresses it resolved from hostnames, while the second contains the
hostnames it resolved from IP addresses. The third added column contains "yes" if there is valid roundtrip /
[FCrDNS](https://en.wikipedia.org/wiki/Forward-confirmed_reverse_DNS) for the row.

If there are any input rows that can't be parsed as hostnames or IP addresses they will be discarded,
or written to the file given by the --discards flag.

## Bugs

This is a fairly quick hack for my own use rather than production grade code. Patches or pull requests welcome.