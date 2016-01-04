package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"github.com/kbsriram/dcutils/go/sg"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	overwrite = flag.Bool("overwrite", false, "overwrite any existing gpx file")
)

func usage() {
	fmt.Fprintln(os.Stderr, "usage: sggps [flags] [path ...]\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	flag.Parse()
	for i := 0; i < flag.NArg(); i++ {
		if err := process(flag.Arg(i)); err != nil {
			log.Fatal(err)
		}
	}
}

func process(movPath string) error {
	ext := filepath.Ext(movPath)
	if !strings.EqualFold(ext, ".mov") {
		return errors.New(fmt.Sprintf("%v: Does not end with .MOV", movPath))
	}
	gpxPath := movPath[:len(movPath)-len(ext)] + ".gpx"
	// If we shouldn't overwrite existing files, verify there
	// is no existing gpx file
	if !*overwrite {
		if _, err := os.Stat(gpxPath); err == nil {
			return errors.New(fmt.Sprintf("%v: already exists. Use -overwrite to overwrite it anyway", gpxPath))
		}
	}
	movFile, err := os.Open(movPath)
	if err != nil {
		return err
	}
	defer movFile.Close()
	gpxFile, err := os.Create(gpxPath)
	if err != nil {
		return err
	}
	defer gpxFile.Close()
	out := bufio.NewWriter(gpxFile)
	defer out.Flush()

	sgInfo := sg.NewInfo(movFile)
	gpsLogs, err := sgInfo.GPSLogs()
	if err != nil {
		return err
	}
	writeHeader(out)
	for _, gpsLog := range gpsLogs {
		if e := writePoint(out, &gpsLog); e != nil {
			return e
		}
	}
	writeFooter(out)
	return nil
}

func writePoint(w *bufio.Writer, gpsLog *sg.GPSLog) error {
	w.WriteString(fmt.Sprintf(`
      <trkpt lat="%.6f" lon="%.6f">`,
		toDD(gpsLog.LatitudeSpec, gpsLog.Latitude),
		toDD(gpsLog.LongitudeSpec, gpsLog.Longitude)))
	
	// assumes timestamps are in localtime
	_, offset := time.Now().Zone()
	var neg byte
	if offset < 0 {
		neg = '-'
		offset = -offset
	} else {
		neg = '+'
	}

	hOffset := offset / 3600
	sOffset := offset % 3600

	w.WriteString(fmt.Sprintf(`
        <time>%4d-%02d-%02dT%02d:%02d:%02d%c%02d:%02d</time>`,
		2000 + int(gpsLog.Year), int(gpsLog.Mon), int(gpsLog.Day),
		int(gpsLog.Hour), int(gpsLog.Min), int(gpsLog.Sec),
		neg, hOffset, sOffset / 60))

	w.WriteString(`
        <extensions>
          <gpxtpx:TrackPointExtension>`)

	// speed comes in knots, convert to m/s
	w.WriteString(fmt.Sprintf(`
            <gpxtpx:speed>%.6f</gpxtpx:speed>`,
	gpsLog.Speed * 0.514444))

	// At low speeds, 0 values for bearing appear to mean unknown, so
	// skip such points
	if gpsLog.Speed > 2 || gpsLog.Bearing > 0.00001 {
		w.WriteString(fmt.Sprintf(`
            <gpxtpx:course>%.6f</gpxtpx:course>`,
			gpsLog.Bearing))
	}

	w.WriteString(`
          </gpxtpx:TrackPointExtension>
        </extensions>`)

	w.WriteString(`
      </trkpt>`)
	return nil
}

// Input comes as decimal minutes
func toDD(spec byte, v float32) float32 {
	deg, frac := math.Modf(float64(v)/100)
	result := float32(deg + frac/0.6)
	// An educated guess
	if spec == 'S' || spec == 'W' {
		result = -result
	}
	return result
}

func writeHeader(w *bufio.Writer) error {
	_, err := w.WriteString(`<?xml version="1.0" encoding="UTF-8" ?>
<gpx
 xmlns="http://www.topografix.com/GPX/1/1"
 xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
 xsi:schemaLocation="http://www.topografix.com/GPX/1/1 http://www.topografix.com/GPX/1/1/gpx.xsd"
 xmlns:gpxtpx="http://www.garmin.com/xmlschemas/TrackPointExtension/v2"
 version="1.1"
 creator="https://github.com/kbsriram/dcutils">
  <trk>
    <trkseg>`)
	return err
}

func writeFooter(w *bufio.Writer) error {
	_, err := w.WriteString(`
    </trkseg>
  </trk>
</gpx>
`)
	return err
}
