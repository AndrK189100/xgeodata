package main

import (
	"bufio"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
)

type DomainEntry struct {
	Pref int
	Dom  string
}

type CidrEntry struct {
	Pref uint32
	Ip   []byte
}

type Target struct {
	Name          string
	DomainEntries []DomainEntry
	CidrEntries   []CidrEntry
}

// scanLines calls fn for every non-empty line with comments stripped.
// Errors returned by fn are prefixed with file path and line number.
func scanLines(filePath string, fn func(line string) error) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++

		line, _, _ := strings.Cut(scanner.Text(), "#")
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		if err := fn(line); err != nil {
			return fmt.Errorf("%s:%d: %w", filePath, lineNum, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("%s: read failed after line %d: %w", filePath, lineNum, err)
	}

	return nil
}

func readLinesCidr(filePath string) ([]CidrEntry, error) {
	var cidrEntries []CidrEntry

	err := scanLines(filePath, func(line string) error {
		raw := line

		if !strings.Contains(line, "/") {
			if !strings.Contains(line, ":") {
				line += "/32"
			} else {
				line += "/128"
			}
		}

		cidr, err := netip.ParsePrefix(line)
		if err != nil {
			return fmt.Errorf("invalid IP or CIDR %q: %w", raw, err)
		}

		cidrEntries = append(cidrEntries, CidrEntry{
			Pref: uint32(cidr.Bits()),
			Ip:   cidr.Addr().AsSlice(),
		})
		return nil
	})

	return cidrEntries, err
}

func readLinesDomain(filePath string) ([]DomainEntry, error) {
	var domainEntries []DomainEntry

	err := scanLines(filePath, func(line string) error {
		pref, dom, found := strings.Cut(line, ":")

		if !found {
			domainEntries = append(domainEntries, DomainEntry{Pref: 2, Dom: pref})
			return nil
		}

		if dom == "" {
			return fmt.Errorf("empty value after prefix %q in %q", pref+":", line)
		}

		switch pref {
		case "keyword":
			domainEntries = append(domainEntries, DomainEntry{Pref: 0, Dom: dom})
		case "regexp":
			domainEntries = append(domainEntries, DomainEntry{Pref: 1, Dom: dom})
		case "domain":
			domainEntries = append(domainEntries, DomainEntry{Pref: 2, Dom: dom})
		case "full":
			domainEntries = append(domainEntries, DomainEntry{Pref: 3, Dom: dom})
		default:
			return fmt.Errorf("unknown prefix %q in %q (expected keyword:, regexp:, domain:, full:)", pref, line)
		}
		return nil
	})

	return domainEntries, err
}

func readFiles(dirPath string) ([]Target, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("read input dir %q: %w", dirPath, err)
	}

	var result []Target

	for _, entry := range entries {
		var domainEntries []DomainEntry
		var cidrEntries []CidrEntry

		if entry.IsDir() {
			continue
		}

		rawName := entry.Name()

		baseName, extension, found := strings.Cut(rawName, ".")
		if !found {
			continue
		}

		fullPath := filepath.Join(dirPath, rawName)

		switch extension {
		case "domain":
			domainEntries, err = readLinesDomain(fullPath)
		case "cidr":
			cidrEntries, err = readLinesCidr(fullPath)
		default:
			continue
		}

		if err != nil {
			return nil, err
		}

		if len(domainEntries) == 0 && len(cidrEntries) == 0 {
			continue
		}

		result = append(result, Target{
			Name:          baseName,
			DomainEntries: domainEntries,
			CidrEntries:   cidrEntries,
		})
	}

	return result, nil
}
