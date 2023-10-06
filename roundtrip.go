package main

import (
	"encoding/csv"
	"fmt"
	"github.com/carlmjohnson/versioninfo"
	"github.com/schollz/progressbar/v3"
	flag "github.com/spf13/pflag"
	"net"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

var inputColumn string
var help bool
var version bool
var discards string
var forceColor = false

func main() {

	flag.BoolVarP(&help, "help", "h", false, "Print this help and exit")
	flag.BoolVar(&version, "version", false, "Print version information and exit")
	flag.StringVar(&inputColumn, "column", "1", "Look for addresses or hostnames in this column")
	flag.StringVarP(&outputFilename, "out", "o", "", "Send output CSV to this file")
	flag.StringVar(&discards, "discards", "", "Write bad input lines to this csv file")
	//flag.StringVar(&dnsServer, "dns", "", "Use this DNS server")
	flag.Parse()

	if help {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: roundtrip [--out=out.csv] [file.csv]\n%s\n", flag.CommandLine.FlagUsages())
		os.Exit(0)
	}

	if version {
		fmt.Println("Version:", versioninfo.Version)
		fmt.Println("Revision:", versioninfo.Revision)
		if versioninfo.Revision != "unknown" {
			fmt.Println("Committed:", versioninfo.LastCommit.Format(time.RFC1123))
			if versioninfo.DirtyBuild {
				fmt.Println("Dirty Build")
			}
		}
		os.Exit(0)
	}

	in, err := openInput(flag.Args())
	if err != nil {
		ErrorExit(err)
	}
	reader := csv.NewReader(in)
	records, err := reader.ReadAll()
	if err != nil {
		ErrorExit(err)
	}
	Success("Read %d lines\n", len(records))
	process(records)

}

func process(records [][]string) {
	colNames := records[0]
	records = records[1:]
	for i, v := range records {
		if len(v) != len(colNames) {
			ErrorExit(fmt.Errorf("record %d has %d columns, I was expecting %d\n%s\n", i+2, len(v), len(colNames), strings.Join(v, ",")))
		}
	}
	inCol := -1
	val, err := strconv.Atoi(inputColumn)
	if err == nil {
		inCol = val - 1
	} else {
		for i, v := range colNames {
			if v == inputColumn {
				inCol = i
			}
		}
	}
	if inCol == -1 {
		ErrorExit(fmt.Errorf("can't find input column '%s'", inputColumn))
	}
	if inCol < 0 || inCol >= len(colNames) {
		ErrorExit(fmt.Errorf("input column %d is out of range", inCol+1))
	}
	Success("using column %d (%s) as input column\n", inCol+1, colNames[inCol])

	newColNames := colNames
	for _, colName := range []string{"ips_from_forward_dns", "hostnames_from_reverse_dns", "roundtrip_ok"} {
		if !slices.Contains(newColNames, colName) {
			newColNames = append(newColNames, colName)
			continue
		}
		spin := 1
		for slices.Contains(newColNames, fmt.Sprintf("%s_%d", colName, spin)) {
			spin++
		}
		newColNames = append(newColNames, fmt.Sprintf("%s_%d", colName, spin))
	}

	var discardedRows [][]string
	var processedRows [][]string

	// Do actual work

	hostnameRe := regexp.MustCompile(`^[a-zA-Z0-9_-]{1,63}(?:\.[a-zA-Z0-9_-]{1,63})+\.?$`)

	bar := progressbar.NewOptions(len(records),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionSetRenderBlankState(true))
	for lineNum, record := range records {
		input := strings.TrimSpace(record[inCol])
		ip := net.ParseIP(input)
		_ = bar.Set(lineNum)
		bar.Describe(input)
		var forward, reverse string
		var ok bool
		if ip != nil {
			forward, reverse, ok = handleIP(ip.String())
		} else if hostnameRe.MatchString(input) {
			forward, reverse, ok = handleName(input)
		} else {
			discardedRows = append(discardedRows, record)
			continue
		}
		var roundtripOK string
		if ok {
			roundtripOK = "yes"
		} else {
			roundtripOK = "no"
		}
		processedRows = append(processedRows, append(record, forward, reverse, roundtripOK))
	}
	_ = bar.Finish()

	// Write output
	outf, err := outputFile()
	if err != nil {
		ErrorExit(fmt.Errorf("while creating output: %w", err))
	}
	writer := csv.NewWriter(outf)
	err = writer.Write(newColNames)
	if err != nil {
		ErrorExit(err)
	}
	err = writer.WriteAll(processedRows)
	if err != nil {
		ErrorExit(err)
	}
	Success("%d liness written to %s\n", len(processedRows)+1, outputFilePretty())
	if len(discardedRows) > 0 {
		if discards == "" {
			Warn("%d rows discarded (use --discards to see them)\n", len(discardedRows))
		} else {
			w, err := os.Create(discards)
			if err != nil {
				ErrorExit(err)
			}
			writer := csv.NewWriter(w)
			err = writer.Write(colNames)
			if err != nil {
				ErrorExit(fmt.Errorf("while writing discards: %w", err))
			}
			err = writer.WriteAll(discardedRows)
			if err != nil {
				ErrorExit(fmt.Errorf("while writing discards: %w", err))
			}
			Warn("%d discarded rows written to %s\n", len(discardedRows), discards)
		}
	}
}

func handleIP(ip string) (string, string, bool) {
	names, err := net.LookupAddr(ip)

	if err != nil {
		return "", fmt.Sprintf("ERROR: %s", err), false
	}
	for i, v := range names {
		names[i] = strings.TrimSuffix(strings.ToLower(v), ".")
	}
	slices.Sort(names)
	slices.Compact(names)
	ips := map[string]struct{}{}
	for _, name := range names {
		addrs, err := net.LookupHost(name)
		if err != nil {
			return fmt.Sprintf("ERROR: %s", err), "", false
		}
		for _, addr := range addrs {
			ips[addr] = struct{}{}
		}
	}
	_, ok := ips[ip]
	var addresses []string
	for ipFound := range ips {
		addresses = append(addresses, ipFound)
	}
	slices.Sort(addresses)
	return strings.Join(addresses, " "), strings.Join(names, " "), ok
}

func handleName(name string) (string, string, bool) {
	name = strings.TrimSuffix(strings.ToLower(name), ".")
	addrs, err := net.LookupHost(name)
	if err != nil {
		return fmt.Sprintf("ERROR: %s", err), "", false
	}
	slices.Sort(addrs)
	slices.Compact(addrs)
	hostnames := map[string]struct{}{}
	for _, addr := range addrs {
		names, err := net.LookupAddr(addr)
		if err != nil {
			return "", fmt.Sprintf("ERROR: %s", err), false
		}
		for _, nameFound := range names {
			hostnames[strings.TrimSuffix(strings.ToLower(nameFound), ".")] = struct{}{}
		}
	}
	_, ok := hostnames[name]
	var hosts []string
	for host := range hostnames {
		hosts = append(hosts, host)
	}
	slices.Sort(hosts)
	return strings.Join(addrs, " "), strings.Join(hosts, " "), ok
}
