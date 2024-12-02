// HLS utils

package main

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	M3U8_DEFAULT_VERSION     = 3
	HLS_DEFAULT_SEGMENT_TIME = 3
)

// Stores a HLS playlist
type HLS_PlayList struct {
	Version        int  // M3U8 version
	TargetDuration int  // Fragment duration
	MediaSequence  int  // First fragment index
	IsVOD          bool // True if the playlist is a VOD playlist
	IsEnded        bool // True if the playlist is an ended playlist

	fragments []HLS_Fragment // Video TS fragments
}

// Stores HLS fragment metadata
type HLS_Fragment struct {
	Index        int     // Fragment index
	Duration     float64 // Fragment duration
	FragmentName string  // Fragment file name
}

// Encodes playlist to M3U8
func (playlist *HLS_PlayList) Encode() string {
	result := "#EXTM3U" + "\n"

	if playlist.IsVOD {
		result += "#EXT-X-PLAYLIST-TYPE:VOD" + "\n"
	}

	result += "#EXT-X-VERSION:" + fmt.Sprint(playlist.Version) + "\n"
	result += "#EXT-X-TARGETDURATION:" + fmt.Sprint(playlist.TargetDuration) + "\n"
	result += "#EXT-X-MEDIA-SEQUENCE:" + fmt.Sprint(playlist.MediaSequence) + "\n"

	for i := 0; i < len(playlist.fragments); i++ {
		result += "#EXTINF:" + fmt.Sprintf("%0.6f", playlist.fragments[i].Duration) + "," + "\n"
		result += playlist.fragments[i].FragmentName + "\n"
	}

	if playlist.IsEnded {
		result += "#EXT-X-ENDLIST" + "\n"
	}

	return result
}

// Decodes HLS playlist
// m3u8 - Content of the .m3u8 file
// Returns the playlist data
func DecodeHLSPlayList(m3u8 string) *HLS_PlayList {
	result := &HLS_PlayList{
		Version:        M3U8_DEFAULT_VERSION,
		TargetDuration: HLS_DEFAULT_SEGMENT_TIME,
		MediaSequence:  0,
		IsVOD:          false,
		IsEnded:        false,
		fragments:      make([]HLS_Fragment, 0),
	}

	lines := strings.Split(m3u8, "\n")

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "#") {
			continue
		}

		if line == "#EXT-X-ENDLIST" {
			result.IsEnded = true
			continue
		}

		parts := strings.Split(line, ":")

		if len(parts) != 2 {
			continue
		}

		switch strings.ToUpper(parts[0]) {
		case "#EXT-X-PLAYLIST-TYPE":
			if strings.ToUpper(parts[1]) == "VOD" {
				result.IsVOD = true
			}
		case "#EXT-X-VERSION":
			v, err := strconv.Atoi(parts[1])
			if err == nil && v >= 0 {
				result.Version = v
			}
		case "#EXT-X-TARGETDURATION":
			td, err := strconv.Atoi(parts[1])
			if err == nil && td > 0 {
				result.TargetDuration = td
			}
		case "#EXT-X-MEDIA-SEQUENCE":
			ms, err := strconv.Atoi(parts[1])
			if err == nil && ms >= 0 {
				result.MediaSequence = ms
			}
		case "#EXTINF":
			d, err := strconv.ParseFloat(strings.TrimSuffix(parts[1], ","), 64)

			if err == nil && d > 0 && i < (len(lines)-1) {
				frag := HLS_Fragment{
					Index:        len(result.fragments) + result.MediaSequence,
					Duration:     d,
					FragmentName: strings.TrimSpace(lines[i+1]),
				}

				result.fragments = append(result.fragments, frag)
			}
		}
	}

	return result
}
