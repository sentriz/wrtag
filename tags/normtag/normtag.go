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
	Album                     = "ALBUM"                      //tag: alts "TALB" "©ALB" "TAL"
	Date                      = "DATE"                       //tag: alts "YEAR" "RELEASEDATE" "TDRC" "TYER" "TDAT" "©DAY" "TYE"
	OriginalDate              = "ORIGINALDATE"               //tag: alts "ORIGINAL_DATE" "ORIGINAL_YEAR" "TDOR" "TORY"
	MediaFormat               = "MEDIA"                      //tag: alts "TMED"
	Label                     = "LABEL"                      //tag: alts "TPUB"
	CatalogueNum              = "CATALOGNUMBER"              //tag: alts "CATALOG_NUMBER" "CATALOGNUM" "CAT#" "CATALOGID" "CATNUM"
	Barcode                   = "BARCODE"                    //tag: alts "UPC" "MCN"
	Compilation               = "COMPILATION"                //tag: alts "TCMP" "CPIL"
	ReleaseType               = "RELEASETYPE"                //tag: alts "RELEASE_TYPE"
	MusicBrainzAlbumComment   = "MUSICBRAINZ_ALBUMCOMMENT"   //tag: alts "MUSICBRAINZ_ALBUM_COMMENT" "MUSICBRAINZ_RELEASECOMMENT" "MUSICBRAINZ_RELEASE_COMMENT"
	MusicBrainzReleaseID      = "MUSICBRAINZ_ALBUMID"        //tag: alts "MUSICBRAINZ_ALBUM_ID" "MUSICBRAINZ_RELEASEID" "MUSICBRAINZ_RELEASE_ID"
	MusicBrainzReleaseGroupID = "MUSICBRAINZ_RELEASEGROUPID" //tag: alts "MUSICBRAINZ_RELEASEGROUP_ID" "MUSICBRAINZ_RELEASE_GROUP_ID"

	Title                  = "TITLE" //tag: alts "TIT2" "©NAM" "TT2"
	Genre                  = "GENRE" //tag: alts "TCON" "©GEN" "TCO"
	Genres                 = "GENRES"
	TrackNumber            = "TRACKNUMBER"                //tag: alts "TRACK_NUMBER" "TRACK" "TRACKNUM" "TRCK" "TRKN" "TRK"
	TrackTotal             = "TRACKTOTAL"                 //tag: alts "TRACK_TOTAL" "TOTALTRACKS" "TOTALTRACK"
	DiscNumber             = "DISCNUMBER"                 //tag: alts "DISC_NUMBER" "DISC" "TPOS" "DISK" "TPA"
	DiscTotal              = "DISCTOTAL"                  //tag: alts "DISC_TOTAL" "TOTALDISCS" "TOTALDISKS" "TOTALDISC" "TOTALDISK"
	DiscSubtitle           = "DISCSUBTITLE"               //tag: alts "DISC_SUBTITLE" "SETSUBTITLE" "TSST"
	ISRC                   = "ISRC"                       //tag: alts "TSRC"
	MusicBrainzTrackID     = "MUSICBRAINZ_RELEASETRACKID" //tag: alts "MUSICBRAINZ_RELEASETRACK_ID" "MUSICBRAINZ_RELEASE_TRACK_ID"
	MusicBrainzRecordingID = "MUSICBRAINZ_TRACKID"        //tag: alts "MUSICBRAINZ_TRACK_ID" "MUSICBRAINZ_RECORDINGID" "MUSICBRAINZ_RECORDING_ID"

	AlbumArtist              = "ALBUMARTIST"               //tag: alts "ALBUM_ARTIST" "TPE2" "AART" "TP2"
	AlbumArtists             = "ALBUMARTISTS"              //tag: alts "ALBUM_ARTISTS"
	AlbumArtistCredit        = "ALBUMARTIST_CREDIT"        //tag: alts "ALBUM_ARTIST_CREDIT"
	AlbumArtistsCredit       = "ALBUMARTISTS_CREDIT"       //tag: alts "ALBUM_ARTISTS_CREDIT"
	MusicBrainzAlbumArtistID = "MUSICBRAINZ_ALBUMARTISTID" //tag: alts "MUSICBRAINZ_ALBUMARTIST_ID" "MUSICBRAINZ_ALBUM_ARTIST_ID" "MUSICBRAINZ_RELEASEARTISTID" "MUSICBRAINZ_RELEASE_ARTIST_ID"

	Arranger              = "ARRANGER" //tag: alts "TIPL:arranger" "IPLS:arranger"
	Arrangers             = "ARRANGERS"
	ArrangerCredit        = "ARRANGER_CREDIT"
	ArrangersCredit       = "ARRANGERS_CREDIT"
	MusicBrainzArrangerID = "MUSICBRAINZ_ARRANGERID" //tag: alts "MUSICBRAINZ_ARRANGER_ID"

	Artist              = "ARTIST" //tag: alts "TPE1" "©ART" "TP1"
	Artists             = "ARTISTS"
	ArtistCredit        = "ARTIST_CREDIT"        //tag: alts "ARTISTCREDIT"
	ArtistsCredit       = "ARTISTS_CREDIT"       //tag: alts "ARTISTSCREDIT"
	MusicBrainzArtistID = "MUSICBRAINZ_ARTISTID" //tag: alts "MUSICBRAINZ_ARTIST_ID"

	Composer              = "COMPOSER" //tag: alts "TCOM" "©WRT" "TCM"
	Composers             = "COMPOSERS"
	ComposerCredit        = "COMPOSER_CREDIT"
	ComposersCredit       = "COMPOSERS_CREDIT"
	MusicBrainzComposerID = "MUSICBRAINZ_COMPOSERID" //tag: alts "MUSICBRAINZ_COMPOSER_ID"

	Conductor              = "CONDUCTOR" //tag: alts "TPE3" "TP3"
	Conductors             = "CONDUCTORS"
	ConductorCredit        = "CONDUCTOR_CREDIT"
	ConductorsCredit       = "CONDUCTORS_CREDIT"
	MusicBrainzConductorID = "MUSICBRAINZ_CONDUCTORID" //tag: alts "MUSICBRAINZ_CONDUCTOR_ID"

	Engineer              = "ENGINEER" //tag: alts "TIPL:engineer" "IPLS:engineer"
	Engineers             = "ENGINEERS"
	EngineerCredit        = "ENGINEER_CREDIT"
	EngineersCredit       = "ENGINEERS_CREDIT"
	MusicBrainzEngineerID = "MUSICBRAINZ_ENGINEERID" //tag: alts "MUSICBRAINZ_ENGINEER_ID"

	Lyricist              = "LYRICIST" //tag: alts "TEXT" "TXT" "TOLY"
	Lyricists             = "LYRICISTS"
	LyricistCredit        = "LYRICIST_CREDIT"
	LyricistsCredit       = "LYRICISTS_CREDIT"
	MusicBrainzLyricistID = "MUSICBRAINZ_LYRICISTID" //tag: alts "MUSICBRAINZ_LYRICIST_ID"

	Performer              = "PERFORMER" //tag: alts "TMCL:instrument" "IPLS:instrument"
	Performers             = "PERFORMERS"
	PerformerCredit        = "PERFORMER_CREDIT"
	PerformersCredit       = "PERFORMERS_CREDIT"
	MusicBrainzPerformerID = "MUSICBRAINZ_PERFORMERID" //tag: alts "MUSICBRAINZ_PERFORMER_ID"

	Producer              = "PRODUCER" //tag: alts "TIPL:producer" "IPLS:producer"
	Producers             = "PRODUCERS"
	ProducerCredit        = "PRODUCER_CREDIT"
	ProducersCredit       = "PRODUCERS_CREDIT"
	MusicBrainzProducerID = "MUSICBRAINZ_PRODUCERID" //tag: alts "MUSICBRAINZ_PRODUCER_ID"

	Remixer              = "REMIXER" //tag: alts "TPE4"
	Remixers             = "REMIXERS"
	RemixerCredit        = "REMIXER_CREDIT"
	RemixersCredit       = "REMIXERS_CREDIT"
	MusicBrainzRemixerID = "MUSICBRAINZ_REMIXERID" //tag: alts "MUSICBRAINZ_REMIXER_ID"

	Writer              = "WRITER" //tag: alts "TXXX:Writer"
	Writers             = "WRITERS"
	WriterCredit        = "WRITER_CREDIT"
	WritersCredit       = "WRITERS_CREDIT"
	MusicBrainzWriterID = "MUSICBRAINZ_WRITERID" //tag: alts "MUSICBRAINZ_WRITER_ID"

	ReplayGainTrackGain         = "REPLAYGAIN_TRACK_GAIN"
	ReplayGainTrackPeak         = "REPLAYGAIN_TRACK_PEAK"
	ReplayGainTrackRange        = "REPLAYGAIN_TRACK_RANGE"
	ReplayGainAlbumGain         = "REPLAYGAIN_ALBUM_GAIN"
	ReplayGainAlbumPeak         = "REPLAYGAIN_ALBUM_PEAK"
	ReplayGainAlbumRange        = "REPLAYGAIN_ALBUM_RANGE"
	ReplayGainReferenceLoudness = "REPLAYGAIN_REFERENCE_LOUDNESS"

	BPM = "BPM"        //tag: alts "TBPM" "TMPO" "TBP"
	Key = "INITIALKEY" //tag: alts "INITIAL_KEY" "TKEY" "TKE"

	Lyrics = "LYRICS" //tag: alts "LYRICS:DESCRIPTION" "USLT:DESCRIPTION" "©LYR" "USLT" "ULT"

	AcoustIDID          = "ACOUSTID_ID"          //tag: alts "TXXX:Acoustid Id"
	AcoustIDFingerprint = "ACOUSTID_FINGERPRINT" //tag: alts "TXXX:Acoustid Fingerprint"

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
