#!/usr/bin/env bash

function _wrtag_completion() {
  # because splitting at spaces is dumb
  IFS=$'\n'

  # slice of COMP_WORDS from the start (excluding the command itself) to the word of where the cursor is
  PREV_COMP_WORDS=("${COMP_WORDS[@]:1:$((COMP_CWORD + 1))}")

  declare -a opts

  # we haven't found a command yet so complete options before command
  if [[ ! "${IFS}${PREV_COMP_WORDS[*]}${IFS}" =~ ${IFS}(move|copy|reflink|sync)${IFS} ]]; then
    # complete gnu-style options if the current completion starts with --
    if [[ "${PREV_COMP_WORDS[-1]}" =~ ^--.*$ ]]; then
      opts=(
        --h
        --help
        --addon
        --caa-base-url
        --caa-rate-limit
        --config
        --config-path
        --cover-upgrade
        --diff-weight
        --keep-file
        --log-level
        --mb-base-url
        --mb-rate-limit
        --notification-uri
        --path-format
        --research-link
        --tag-config
        --version
      )
    else
      opts=(
        -h
        -help
        -addon
        -caa-base-url
        -caa-rate-limit
        -config
        -config-path
        -cover-upgrade
        -diff-weight
        -keep-file
        -log-level
        -mb-base-url
        -mb-rate-limit
        -notification-uri
        -path-format
        -research-link
        -tag-config
        -version
        move
        copy
        reflink
        sync
      )
    fi
  # complete options for operation commands
  elif [[ "${IFS}${PREV_COMP_WORDS[*]}${IFS}" =~ ${IFS}(move|copy|reflink)${IFS} ]]; then
    # complete gnu-style options if the current completion starts with --
    if [[ "${PREV_COMP_WORDS[-1]}" =~ ^--.*$ ]]; then
      opts=(
        --h
        --help
        --dry-run
        --mbid
        --yes
      )
    else
      opts=(
        -h
        -help
        -dry-run
        -mbid
        -yes
      )
    fi
  # complete options for sync commands
  elif [[ "${IFS}${PREV_COMP_WORDS[*]}${IFS}" =~ ${IFS}sync${IFS} ]]; then
    # complete gnu-style options if the current completion starts with --
    if [[ "${PREV_COMP_WORDS[-1]}" =~ ^--.*$ ]]; then
      opts=(
        --h
        --help
        --age-older
        --age-younger
        --dry-run
        --num-workers
      )
    else
      opts=(
        -h
        -help
        -age-older
        -age-younger
        -dry-run
        -num-workers
      )
    fi
  fi

  readarray -d $'\0' COMPREPLY < <(printf "%s\0" "${opts[@]}" | grep -zx -- "${PREV_COMP_WORDS[-1]}.*")
}

complete -F _wrtag_completion wrtag
