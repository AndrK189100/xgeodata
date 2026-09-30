package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"xgeodata/geodata"

	"google.golang.org/protobuf/proto"
)

func saveGeoData(targets []Target, dirPath string) error {
	geoSiteList := &geodata.GeoSiteList{}
	geoIpList := &geodata.GeoIPList{}
	domainCount, cidrCount := 0, 0

	for _, target := range targets {
		if len(target.DomainEntries) > 0 {
			siteEntry := &geodata.GeoSite{
				Code: strings.ToUpper(target.Name),
			}

			for _, domain := range target.DomainEntries {
				siteEntry.Domain = append(siteEntry.Domain, &geodata.Domain{
					Type:  geodata.Domain_Type(domain.Pref),
					Value: domain.Dom,
				})
			}
			geoSiteList.Entry = append(geoSiteList.Entry, siteEntry)
			domainCount += len(target.DomainEntries)
		}

		if len(target.CidrEntries) > 0 {
			cidrEntry := &geodata.GeoIP{
				Code: strings.ToUpper(target.Name),
			}

			for _, cidr := range target.CidrEntries {
				cidrEntry.Cidr = append(cidrEntry.Cidr, &geodata.CIDR{
					Ip:     cidr.Ip,
					Prefix: cidr.Pref,
				})
			}
			geoIpList.Entry = append(geoIpList.Entry, cidrEntry)
			cidrCount += len(target.CidrEntries)
		}
	}

	if len(geoSiteList.Entry) > 0 {
		if err := writeProto(geoSiteList, filepath.Join(dirPath, "geosite.dat")); err != nil {
			return err
		}
		log.Printf("geosite.dat written: %d tags, %d domains", len(geoSiteList.Entry), domainCount)
	}

	if len(geoIpList.Entry) > 0 {
		if err := writeProto(geoIpList, filepath.Join(dirPath, "geoip.dat")); err != nil {
			return err
		}
		log.Printf("geoip.dat written: %d tags, %d CIDRs", len(geoIpList.Entry), cidrCount)
	}

	return nil
}

func writeProto(msg proto.Message, path string) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}
