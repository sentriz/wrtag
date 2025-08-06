#!/usr/bin/env 

function __last_argument_from
    string match -- (commandline -pcx)[-1] $argv
end

function __is_long_option
    string match -r -- '^--.*$' (commandline -px)[-1]
end

set commands read write clear
set tags album artists title genres

# don't suggest files if we haven't seen the end of the options
complete -c metadata -n "not __fish_seen_subcommand_from --" --no-files

# complete global options if we haven't seen a subcommand
complete -c metadata -n "not __fish_seen_subcommand_from $commands" \
    -o h -o help -d "print help"

complete -c metadata -n "not __fish_seen_subcommand_from $commands" \
    -o log-level -x -d "Set the logging level (default INFO)" \
    -a "INFO WARN DEBUG ERROR"

# complete gnu style global options if we haven't seen a subcommand
complete -c metadata -n "not __fish_seen_subcommand_from $commands" \
    -n __is_long_option \
    -l h -l help -d "print help"

complete -c metadata -n "not __fish_seen_subcommand_from $commands" \
    -n __is_long_option \
    -l log-level -x -d "Set the logging level (default INFO)" \
    -a "INFO WARN DEBUG ERROR"

# complete subcommands if we haven't seen a subcommand
complete -c metadata -n "not __fish_seen_subcommand_from $commands" \
    -a read -d "read tags"

complete -c metadata -n "not __fish_seen_subcommand_from $commands" \
    -a write -d "write tags"

complete -c metadata -n "not __fish_seen_subcommand_from $commands" \
    -a clear -d "clear tags"

# complete subcommand options
complete -c metadata -n "__fish_seen_subcommand_from read" \
    -n "not __fish_seen_subcommand_from - --" \
    -o properties -d "Read file properties like length and bitrate"

complete -c metadata -n "__fish_seen_subcommand_from $commands" \
    -n "not __fish_seen_subcommand_from - --" \
    -a "$tags"

complete -c metadata -n "__fish_seen_subcommand_from write" \
    -n "not __fish_seen_subcommand_from - --" \
    -n "not __last_argument_from write ," \
    -a "," -d "delimiter for different tags to write"

complete -c metadata -n "__fish_seen_subcommand_from $commands" \
    -n "not __fish_seen_subcommand_from - --" \
    -a - -d "read list of paths from stdin"

complete -c metadata -n "__fish_seen_subcommand_from $commands" \
    -n "not __fish_seen_subcommand_from - --" \
    -a -- -d "end of options, start of filenames"

# complete gnu style subcommand options
complete -c metadata -n "__fish_seen_subcommand_from read" \
    -n "not __fish_seen_subcommand_from - --" \
    -n __is_long_option \
    -l properties -d "Read file properties like length and bitrate"
