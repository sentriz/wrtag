package essentia

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
)

var ErrNoStreamingExtractorMusic = errors.New("streaming_extractor_music not found in PATH")

const StreamingExtractorMusicCommand = "streaming_extractor_music"

func Read(ctx context.Context, path string) (info *Info, err error) {
	if _, err := exec.LookPath(StreamingExtractorMusicCommand); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoStreamingExtractorMusic, err)
	}

	cmd := exec.CommandContext(ctx, StreamingExtractorMusicCommand, path, "-") //nolint:gosec // args are only args and paths

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	defer func() {
		if err != nil && stderr.Len() > 0 {
			err = fmt.Errorf("%w: stderr: %q", err, stderr.String())
		}
	}()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("get stdout pipe: %w", err)
	}
	defer stdout.Close()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start cmd: %w", err)
	}

	if err := json.NewDecoder(stdout).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}

	return info, nil
}

type Info struct {
	LowLevel struct {
		AverageLoudness              float64 `json:"average_loudness"`
		BarkbandsCrest               Stat    `json:"barkbands_crest"`
		BarkbandsFlatnessDb          Stat    `json:"barkbands_flatness_db"`
		BarkbandsKurtosis            Stat    `json:"barkbands_kurtosis"`
		BarkbandsSkewness            Stat    `json:"barkbands_skewness"`
		BarkbandsSpread              Stat    `json:"barkbands_spread"`
		Dissonance                   Stat    `json:"dissonance"`
		DynamicComplexity            float64 `json:"dynamic_complexity"`
		ErbbandsCrest                Stat    `json:"erbbands_crest"`
		ErbbandsFlatnessDb           Stat    `json:"erbbands_flatness_db"`
		ErbbandsKurtosis             Stat    `json:"erbbands_kurtosis"`
		ErbbandsSkewness             Stat    `json:"erbbands_skewness"`
		ErbbandsSpread               Stat    `json:"erbbands_spread"`
		Hfc                          Stat    `json:"hfc"`
		MelbandsCrest                Stat    `json:"melbands_crest"`
		MelbandsFlatnessDb           Stat    `json:"melbands_flatness_db"`
		MelbandsKurtosis             Stat    `json:"melbands_kurtosis"`
		MelbandsSkewness             Stat    `json:"melbands_skewness"`
		MelbandsSpread               Stat    `json:"melbands_spread"`
		PitchSalience                Stat    `json:"pitch_salience"`
		SilenceRate20DB              Stat    `json:"silence_rate_20dB"`
		SilenceRate30DB              Stat    `json:"silence_rate_30dB"`
		SilenceRate60DB              Stat    `json:"silence_rate_60dB"`
		SpectralCentroid             Stat    `json:"spectral_centroid"`
		SpectralComplexity           Stat    `json:"spectral_complexity"`
		SpectralDecrease             Stat    `json:"spectral_decrease"`
		SpectralEnergy               Stat    `json:"spectral_energy"`
		SpectralEnergybandHigh       Stat    `json:"spectral_energyband_high"`
		SpectralEnergybandLow        Stat    `json:"spectral_energyband_low"`
		SpectralEnergybandMiddleHigh Stat    `json:"spectral_energyband_middle_high"`
		SpectralEnergybandMiddleLow  Stat    `json:"spectral_energyband_middle_low"`
		SpectralEntropy              Stat    `json:"spectral_entropy"`
		SpectralFlux                 Stat    `json:"spectral_flux"`
		SpectralKurtosis             Stat    `json:"spectral_kurtosis"`
		SpectralRms                  Stat    `json:"spectral_rms"`
		SpectralRolloff              Stat    `json:"spectral_rolloff"`
		SpectralSkewness             Stat    `json:"spectral_skewness"`
		SpectralSpread               Stat    `json:"spectral_spread"`
		SpectralStrongpeak           Stat    `json:"spectral_strongpeak"`
		Zerocrossingrate             Stat    `json:"zerocrossingrate"`
		Barkbands                    Bands   `json:"barkbands"`
		Erbbands                     Bands   `json:"erbbands"`
		Gfcc                         struct {
			Mean []float64   `json:"mean"`
			Cov  [][]float64 `json:"cov"`
			Icov [][]float64 `json:"icov"`
		} `json:"gfcc"`
		Melbands Bands `json:"melbands"`
		Mfcc     struct {
			Mean []float64   `json:"mean"`
			Cov  [][]float64 `json:"cov"`
			Icov [][]float64 `json:"icov"`
		} `json:"mfcc"`
		SpectralContrastCoeffs  Bands `json:"spectral_contrast_coeffs"`
		SpectralContrastValleys Bands `json:"spectral_contrast_valleys"`
	} `json:"lowlevel"`
	Metadata struct {
		AudioProperties struct {
			AnalysisSampleRate int     `json:"analysis_sample_rate"`
			BitRate            int     `json:"bit_rate"`
			EqualLoudness      int     `json:"equal_loudness"`
			Length             float64 `json:"length"`
			Lossless           int     `json:"lossless"`
			ReplayGain         float64 `json:"replay_gain"`
			SampleRate         int     `json:"sample_rate"`
			Codec              string  `json:"codec"`
			Downmix            string  `json:"downmix"`
			Md5Encoded         string  `json:"md5_encoded"`
		} `json:"audio_properties"`
		Tags    map[string]any `json:"tags"`
		Version struct {
			Essentia       string `json:"essentia"`
			EssentiaGitSha string `json:"essentia_git_sha"`
			Extractor      string `json:"extractor"`
		} `json:"version"`
	} `json:"metadata"`
	Rhythm struct {
		BeatsCount                   int       `json:"beats_count"`
		BeatsLoudness                Stat      `json:"beats_loudness"`
		BPM                          float64   `json:"bpm"`
		BPMHistogramFirstPeakBPM     Stat      `json:"bpm_histogram_first_peak_bpm"`
		BPMHistogramFirstPeakSpread  Stat      `json:"bpm_histogram_first_peak_spread"`
		BPMHistogramFirstPeakWeight  Stat      `json:"bpm_histogram_first_peak_weight"`
		BPMHistogramSecondPeakBPM    Stat      `json:"bpm_histogram_second_peak_bpm"`
		BPMHistogramSecondPeakSpread Stat      `json:"bpm_histogram_second_peak_spread"`
		BPMHistogramSecondPeakWeight Stat      `json:"bpm_histogram_second_peak_weight"`
		Danceability                 float64   `json:"danceability"`
		OnsetRate                    float64   `json:"onset_rate"`
		BeatsLoudnessBandRatio       Bands     `json:"beats_loudness_band_ratio"`
		BeatsPosition                []float64 `json:"beats_position"`
	} `json:"rhythm"`
	Tonal struct {
		ChordsChangesRate            float64   `json:"chords_changes_rate"`
		ChordsNumberRate             float64   `json:"chords_number_rate"`
		ChordsStrength               Stat      `json:"chords_strength"`
		HpcpEntropy                  Stat      `json:"hpcp_entropy"`
		KeyStrength                  float64   `json:"key_strength"`
		TuningDiatonicStrength       float64   `json:"tuning_diatonic_strength"`
		TuningEqualTemperedDeviation float64   `json:"tuning_equal_tempered_deviation"`
		TuningFrequency              float64   `json:"tuning_frequency"`
		TuningNontemperedEnergyRatio float64   `json:"tuning_nontempered_energy_ratio"`
		Hpcp                         Bands     `json:"hpcp"`
		ChordsHistogram              []float64 `json:"chords_histogram"`
		Thpcp                        []float64 `json:"thpcp"`
		ChordsKey                    string    `json:"chords_key"`
		ChordsScale                  string    `json:"chords_scale"`
		KeyKey                       string    `json:"key_key"`
		KeyScale                     string    `json:"key_scale"`
	} `json:"tonal"`
}

type Stat struct {
	Dmean  float64 `json:"dmean"`
	Dmean2 float64 `json:"dmean2"`
	Dvar   float64 `json:"dvar"`
	Dvar2  float64 `json:"dvar2"`
	Max    float64 `json:"max"`
	Mean   float64 `json:"mean"`
	Median float64 `json:"median"`
	Min    float64 `json:"min"`
	Var    float64 `json:"var"`
}

type Bands struct {
	Dmean  []float64 `json:"dmean"`
	Dmean2 []float64 `json:"dmean2"`
	Dvar   []float64 `json:"dvar"`
	Dvar2  []float64 `json:"dvar2"`
	Max    []float64 `json:"max"`
	Mean   []float64 `json:"mean"`
	Median []float64 `json:"median"`
	Min    []float64 `json:"min"`
	Var    []float64 `json:"var"`
}
