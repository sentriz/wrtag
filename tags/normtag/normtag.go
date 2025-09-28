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
	Album              = "ALBUM"
	AlbumArtist        = "ALBUMARTIST"         //tag: alts "ALBUM_ARTIST"
	AlbumArtists       = "ALBUMARTISTS"        //tag: alts "ALBUM_ARTISTS"
	AlbumArtistCredit  = "ALBUMARTIST_CREDIT"  //tag: alts "ALBUM_ARTIST_CREDIT"
	AlbumArtistsCredit = "ALBUMARTISTS_CREDIT" //tag: alts "ALBUM_ARTISTS_CREDIT"
	Date               = "DATE"                //tag: alts "YEAR" "RELEASEDATE"
	OriginalDate       = "ORIGINALDATE"        //tag: alts "ORIGINAL_YEAR"
	MediaFormat        = "MEDIA"
	Label              = "LABEL"
	CatalogueNum       = "CATALOGNUMBER" //tag: alts "CATALOGNUM" "CAT#" "CATALOGID" "CATNUM"
	Barcode            = "BARCODE"       //tag: alts "UPC" "MCN"
	Compilation        = "COMPILATION"
	ReleaseType        = "RELEASETYPE"

	MusicBrainzReleaseID      = "MUSICBRAINZ_ALBUMID"
	MusicBrainzReleaseGroupID = "MUSICBRAINZ_RELEASEGROUPID"
	MusicBrainzAlbumArtistID  = "MUSICBRAINZ_ALBUMARTISTID"
	MusicBrainzAlbumComment   = "MUSICBRAINZ_ALBUMCOMMENT"

	Title         = "TITLE"
	Artist        = "ARTIST"
	Artists       = "ARTISTS"
	ArtistCredit  = "ARTIST_CREDIT"  //tag: alts "ARTISTCREDIT"
	ArtistsCredit = "ARTISTS_CREDIT" //tag: alts "ARTISTSCREDIT"
	Genre         = "GENRE"
	Genres        = "GENRES"
	TrackNumber   = "TRACKNUMBER" //tag: alts "TRACK" "TRACKNUM"
	DiscNumber    = "DISCNUMBER"

	ISRC = "ISRC"

	Remixer        = "REMIXER"
	Remixers       = "REMIXERS"
	RemixerCredit  = "REMIXER_CREDIT"
	RemixersCredit = "REMIXERS_CREDIT"

	Composer        = "COMPOSER"
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

	BPM = "BPM"
	Key = "INITIALKEY" //tag: alts "INITIAL_KEY"

	Lyrics = "LYRICS" //tag: alts "LYRICS:DESCRIPTION" "USLT:DESCRIPTION" "©LYR"

	AcoustIDFingerprint = "ACOUSTID_FINGERPRINT"
	AcoustIDID          = "ACOUSTID_ID"

	Encoder   = "ENCODER"
	EncodedBy = "ENCODEDBY"

	Comment = "COMMENT"
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
