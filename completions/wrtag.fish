#!/usr/bin/env 

function __last_argument_from
    string match -- (commandline -pcx)[-1] $argv
end

set operations move copy reflink
set commands $operations sync
set addonoptions \
    "lyrics "{genius,musixmatch,"genius musixmatch","musixmatch genius"} \
    replaygain{," "{force,true-peak,"force true-peak","true-peak force"}} \
    musicdesc{," force"} \
    "subproc <path/command> <args>..."

complete -c wrtag --erase

# don't suggest files if we haven't seen a subcommand
complete -c wrtag -n "not __fish_seen_subcommand_from $commands" --no-files

# complete global options if we haven't seen a subcommand
complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o addon -l addon -x -d "Define an addon for extra metadata writing" \
    -a "'$(string join "' '" $addonoptions)'"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o caa-base-url -l caa-base-url -x -d 'CoverArtArchive base URL (default "https://coverartarchive.org/")'

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o caa-rate-limit -l caa-rate-limit -x -d "CoverArtArchive rate limit duration"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o config -l config -d "Print the parsed config and exit"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o config-path -l config-path -rF -d 'Path to config file (default "/$HOME/.config/wrtag/config")'

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o cover-upgrade -l cover-upgrade -d "Fetch new cover art even if it exists locally"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o diff-weight -l diff-weight -x -d "Adjust distance weighting for a tag (0 to ignore)"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o h -o help -l help -d "print help"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o keep-file -l keep-file -x -d "Define an extra file path to keep when moving/copying to root dir"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o log-level -l log-level -x -d "Set the logging level (default INFO)" \
    -a "INFO WARN DEBUG ERROR"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o mb-base-url -l mb-base-url -x -d 'MusicBrainz base URL (default "https://musicbrainz.org/ws/2")'

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o mb-rate-limit -l mb-rate-limit -x -d "MusicBrainz rate limit duration (default 1s)"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o notification-uri -l notification-uri -x -d "Add a shoutrrr notification URI for an event"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o path-format -l path-format -x -d "Path to root music directory including path format rules"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o research-link -l research-link -x -d "Define a helper URL to help find information about an unmatched release"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o tag-config -l tag-config -x -d "Specify tag keep and drop rules when writing new tag revisions" \
    -a "'keep <tag>' 'drop <tag>'"

complete -c wrtag -n "not __fish_seen_subcommand_from $commands" \
    -o version -l version -d "Print the version and exit"

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
    -o dry-run -l dry-run -d "Do a dry run of imports"

# operations
complete -c wrtag -n "__fish_seen_subcommand_from $operations" \
    -o mbid -l mbid -x -d "Overwrite matched MusicBrainz release UUID"

complete -c wrtag -n "__fish_seen_subcommand_from $operations" \
    -o yes -l yes -d "Use the found release anyway despite a low score"

# sync
complete -c wrtag -n "__fish_seen_subcommand_from sync" \
    -o age-older -l age-older -x -d "Maximum duration a release should be left unsynced"

complete -c wrtag -n "__fish_seen_subcommand_from sync" \
    -o age-younger -l age-younger -x -d "Minimum duration a release should be left unsynced"

complete -c wrtag -n "__fish_seen_subcommand_from sync" \
    -o num-workers -l num-workers -x -d "Number of directories to process concurrently (default 4)"
