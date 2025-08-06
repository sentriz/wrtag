#!/usr/bin/env

function __last_argument_from
    string match -- (commandline -pcx)[-1] $argv
end

function __is_long_option
    string match -r -- '^--.*$' (commandline -px)[-1]
end

set operations move copy reflink
set commands $operations sync
set addonoptions \
    "lyrics "{genius,musixmatch,"genius musixmatch","musixmatch genius"} \
    replaygain{," "{force,true-peak,"force true-peak","true-peak force"}} \
    musicdesc{," force"} \
    "subproc <path/command> <args>..."

# don't suggest files if we haven't seen a subcommand
complete -c wrtag -n "not __fish_seen_subcommand_from $commands" --no-files

# complete global options if we haven't seen a subcommand
complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o addon -x -d "Define an addon for extra metadata writing" \
    -a "'$(string join "' '" $addonoptions)'"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o caa-base-url -x -d 'CoverArtArchive base URL (default "https://coverartarchive.org/")'

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o caa-rate-limit -x -d "CoverArtArchive rate limit duration"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o config -d "Print the parsed config and exit"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o config-path -rF -d 'Path to config file (default "/$HOME/.config/wrtag/config")'

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o cover-upgrade -d "Fetch new cover art even if it exists locally"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o diff-weight -x -d "Adjust distance weighting for a tag (0 to ignore)"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o h -o help -d "print help"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o keep-file -x -d "Define an extra file path to keep when moving/copying to root dir"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o log-level -x -d "Set the logging level (default INFO)" \
    -a "INFO WARN DEBUG ERROR"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o mb-base-url -x -d 'MusicBrainz base URL (default "https://musicbrainz.org/ws/2")'

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o mb-rate-limit -x -d "MusicBrainz rate limit duration (default 1s)"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o notification-uri -x -d "Add a shoutrrr notification URI for an event"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o path-format -x -d "Path to root music directory including path format rules"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o research-link -x -d "Define a helper URL to help find information about an unmatched release"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o tag-config -x -d "Specify tag keep and drop rules when writing new tag revisions" \
    -a "'keep <tag>' 'drop <tag>'"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o version -d "Print the version and exit"

# complete gnu style global options if we haven't seen a subcommand
complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -n __is_long_option \
    -l addon -x -d "Define an addon for extra metadata writing" \
    -a "'$(string join "' '" $addonoptions)'"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -n __is_long_option \
    -l caa-base-url -x -d 'CoverArtArchive base URL (default "https://coverartarchive.org/")'

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -n __is_long_option \
    -l caa-rate-limit -x -d "CoverArtArchive rate limit duration"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -n __is_long_option \
    -l config -d "Print the parsed config and exit"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -n __is_long_option \
    -l config-path -rF -d 'Path to config file (default "/$HOME/.config/wrtag/config")'

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -n __is_long_option \
    -l cover-upgrade -d "Fetch new cover art even if it exists locally"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -n __is_long_option \
    -l diff-weight -x -d "Adjust distance weighting for a tag (0 to ignore)"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -n __is_long_option \
    -l h -l help -d "print help"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -n __is_long_option \
    -l keep-file -x -d "Define an extra file path to keep when moving/copying to root dir"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -n __is_long_option \
    -l log-level -x -d "Set the logging level (default INFO)" \
    -a "INFO WARN DEBUG ERROR"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -n __is_long_option \
    -l mb-base-url -x -d 'MusicBrainz base URL (default "https://musicbrainz.org/ws/2")'

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -n __is_long_option \
    -l mb-rate-limit -x -d "MusicBrainz rate limit duration (default 1s)"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -n __is_long_option \
    -l notification-uri -x -d "Add a shoutrrr notification URI for an event"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -n __is_long_option \
    -l path-format -x -d "Path to root music directory including path format rules"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -n __is_long_option \
    -l research-link -x -d "Define a helper URL to help find information about an unmatched release"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -n __is_long_option \
    -l tag-config -x -d "Specify tag keep and drop rules when writing new tag revisions" \
    -a "'keep <tag>' 'drop <tag>'"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -n __is_long_option \
    -l version -d "Print the version and exit"

# complete subcommands if we haven't seen a subcommand
complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -a move -d "Move files from the source to the destination directory"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -a copy -d "Copy files from the source to the destination directory"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -a reflink -d "Create a reflink clone of a file from the source to the destination"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -a sync -d "re-tag in bulk (!! can be destructive !!)"

# complete subcommand options
complete -c wrtag -n "__fish_seen_subcommand_from $commands" \
    -o dry-run -d "Do a dry run of imports"

# operations
complete -c wrtag -n "__fish_seen_subcommand_from $operations" \
    -o mbid -x -d "Overwrite matched MusicBrainz release UUID"

complete -c wrtag -n "__fish_seen_subcommand_from $operations" \
    -o yes -d "Use the found release anyway despite a low score"

# sync
complete -c wrtag -n "__fish_seen_subcommand_from sync" \
    -o age-older -x -d "Maximum duration a release should be left unsynced"

complete -c wrtag -n "__fish_seen_subcommand_from sync" \
    -o age-younger -x -d "Minimum duration a release should be left unsynced"

complete -c wrtag -n "__fish_seen_subcommand_from sync" \
    -o num-workers -x -d "Number of directories to process concurrently (default 4)"

# complete gnu-style subcommand options
complete -c wrtag -n "__fish_seen_subcommand_from $commands" \
    -n __is_long_option \
    -l dry-run -d "Do a dry run of imports"

# operations
complete -c wrtag -n "__fish_seen_subcommand_from $operations" \
    -n __is_long_option \
    -l mbid -x -d "Overwrite matched MusicBrainz release UUID"

complete -c wrtag -n "__fish_seen_subcommand_from $operations" \
    -n __is_long_option \
    -l yes -d "Use the found release anyway despite a low score"

# sync
complete -c wrtag -n "__fish_seen_subcommand_from sync" \
    -n __is_long_option \
    -l age-older -x -d "Maximum duration a release should be left unsynced"

complete -c wrtag -n "__fish_seen_subcommand_from sync" \
    -n __is_long_option \
    -l age-younger -x -d "Minimum duration a release should be left unsynced"

complete -c wrtag -n "__fish_seen_subcommand_from sync" \
    -n __is_long_option \
    -l num-workers -x -d "Number of directories to process concurrently (default 4)"
