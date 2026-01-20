// Package normtag provides normalized tag key mapping for audio file metadata.
// It handles conversion between different tag naming conventions (e.g., ID3v2, Vorbis Comments, MP4)
// and provides a consistent interface for reading and writing tags across different formats.
package normtag

import (
	"maps"
	"strings"
)

// https://taglib.org/api/p_propertymapping.html
// https://picard-docs.musicbrainz.org/downloads/MusicBrainz_Picard_Tag_Map.html

//go:generate go run gen_taglist.go -- taglist.go
//nolint:gosec
const (
	Album              = "ALBUM"               //tag: alts "TALB" "©ALB" "TAL"
	AlbumArtist        = "ALBUMARTIST"         //tag: alts "ALBUM_ARTIST" "TPE2" "AART" "TP2"
	AlbumArtists       = "ALBUMARTISTS"        //tag: alts "ALBUM_ARTISTS"
	AlbumArtistCredit  = "ALBUMARTIST_CREDIT"  //tag: alts "ALBUM_ARTIST_CREDIT"
	AlbumArtistsCredit = "ALBUMARTISTS_CREDIT" //tag: alts "ALBUM_ARTISTS_CREDIT"
	Date               = "DATE"                //tag: alts "YEAR" "RELEASEDATE" "TDRC" "TYER" "TDAT" "©DAY" "TYE"
	OriginalDate       = "ORIGINALDATE"        //tag: alts "ORIGINAL_YEAR" "TDOR" "TORY"
	MediaFormat        = "MEDIA"
	Label              = "LABEL"         //tag: alts "TPUB"
	CatalogueNum       = "CATALOGNUMBER" //tag: alts "CATALOGNUM" "CAT#" "CATALOGID" "CATNUM"
	Barcode            = "BARCODE"       //tag: alts "UPC" "MCN"
	Compilation        = "COMPILATION"   //tag: alts "TCMP" "CPIL"
	ReleaseType        = "RELEASETYPE"

	MusicBrainzReleaseID      = "MUSICBRAINZ_ALBUMID"
	MusicBrainzReleaseGroupID = "MUSICBRAINZ_RELEASEGROUPID"
	MusicBrainzAlbumArtistID  = "MUSICBRAINZ_ALBUMARTISTID"
	MusicBrainzAlbumComment   = "MUSICBRAINZ_ALBUMCOMMENT"

	Title         = "TITLE"  //tag: alts "TIT2" "©NAM" "TT2"
	Artist        = "ARTIST" //tag: alts "TPE1" "©ART" "TP1"
	Artists       = "ARTISTS"
	ArtistCredit  = "ARTIST_CREDIT"  //tag: alts "ARTISTCREDIT"
	ArtistsCredit = "ARTISTS_CREDIT" //tag: alts "ARTISTSCREDIT"
	Genre         = "GENRE"          //tag: alts "TCON" "©GEN" "TCO"
	Genres        = "GENRES"
	TrackNumber   = "TRACKNUMBER"  //tag: alts "TRACK" "TRACKNUM" "TRCK" "TRKN" "TRK"
	TrackTotal    = "TRACKTOTAL"   //tag: alts "TOTALTRACKS" "TOTALTRACK"
	DiscNumber    = "DISCNUMBER"   //tag: alts "DISC" "TPOS" "DISK" "TPA"
	DiscTotal     = "DISCTOTAL"    //tag: alts "TOTALDISCS" "TOTALDISKS" "TOTALDISC" "TOTALDISK"
	DiscSubtitle  = "DISCSUBTITLE" //tag: alts "SETSUBTITLE" "TSST"

	ISRC = "ISRC"

	Remixer        = "REMIXER"
	Remixers       = "REMIXERS"
	RemixerCredit  = "REMIXER_CREDIT"
	RemixersCredit = "REMIXERS_CREDIT"

	Composer        = "COMPOSER" //tag: alts "TCOM" "©WRT" "TCM"
	Composers       = "COMPOSERS"
	ComposerCredit  = "COMPOSER_CREDIT"
	ComposersCredit = "COMPOSERS_CREDIT"

	MusicBrainzRecordingID = "MUSICBRAINZ_TRACKID"
	MusicBrainzTrackID     = "MUSICBRAINZ_RELEASETRACKID"
	MusicBrainzArtistID    = "MUSICBRAINZ_ARTISTID"

	ReplayGainTrackGain         = "REPLAYGAIN_TRACK_GAIN"
	ReplayGainTrackPeak         = "REPLAYGAIN_TRACK_PEAK"
	ReplayGainAlbumGain         = "REPLAYGAIN_ALBUM_GAIN"
	ReplayGainAlbumPeak         = "REPLAYGAIN_ALBUM_PEAK"
	ReplayGainTrackRange        = "REPLAYGAIN_TRACK_RANGE"
	ReplayGainAlbumRange        = "REPLAYGAIN_ALBUM_RANGE"
	ReplayGainReferenceLoudness = "REPLAYGAIN_REFERENCE_LOUDNESS"

	BPM = "BPM"        //tag: alts "TBPM" "TMPO" "TBP"
	Key = "INITIALKEY" //tag: alts "INITIAL_KEY" "TKEY" "TKE"

	Lyrics = "LYRICS" //tag: alts "LYRICS:DESCRIPTION" "USLT:DESCRIPTION" "©LYR" "USLT" "ULT"

	AcoustIDFingerprint = "ACOUSTID_FINGERPRINT"
	AcoustIDID          = "ACOUSTID_ID"

	Encoder   = "ENCODER"   //tag: alts "TSSE" "©TOO" "TSS"
	EncodedBy = "ENCODEDBY" //tag: alts "TENC" "ENCODED_BY" "TEN"

	Comment = "COMMENT" //tag: alts "COMM" "©CMT" "COM"
)

func Set(t map[string][]string, key string, values ...string) {
	normKey := NormKey(key)

	// remove any existing alternative keys that would normalize to the same key
	for k := range t {
		if k != normKey && NormKey(k) == normKey {
			delete(t, k)
		}
	}

	t[normKey] = values
}

func Get(t map[string][]string, key string) string {
	normKey := NormKey(key)
	if vs := t[normKey]; len(vs) > 0 {
		return vs[0]
	}
	if altKey := altKey(t, normKey); altKey != "" {
		if vs := t[altKey]; len(vs) > 0 {
			return vs[0]
		}
	}
	return ""
}

func Values(t map[string][]string, key string) []string {
	normKey := NormKey(key)

	if vs := t[normKey]; vs != nil {
		return vs
	}
	if altKey := altKey(t, normKey); altKey != "" {
		return t[altKey]
	}
	return nil
}

func NormKey(k string) string {
	k = strings.ToUpper(k)
	if nk, ok := alternatives[k]; ok {
		return nk
	}
	return k
}

func KnownTags() map[string]struct{} {
	return maps.Clone(knownTags)
}

func altKey(t map[string][]string, key string) string {
	for rawKey := range t {
		if NormKey(rawKey) == key {
			return rawKey
		}
	}
	return ""
}
