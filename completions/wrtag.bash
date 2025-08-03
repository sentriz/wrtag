#!/usr/bin/env bash

function _wrtag_completion() {
  # because splitting at spaces is dumb
  IFS=$'\n'

  # slice of COMP_WORDS from the start (excluding the command itself) to the word of where the cursor is
  PREV_COMP_WORDS=("${COMP_WORDS[@]:1:$((COMP_CWORD + 1))}")

  declare -a opts

  # if we haven't found a command yet so complete options before command
  if [[ ! "${IFS}${PREV_COMP_WORDS[*]}${IFS}" =~ ${IFS}(move|copy|reflink|sync)${IFS} ]]; then
    opts=(
      -h
      -help
      --help
      -addon
      --addon
      -caa-base-url
      --caa-base-url
      -caa-rate-limit
      --caa-rate-limit
      -config
      --config
      -config-path
      --config-path
      -cover-upgrade
      --cover-upgrade
      -diff-weight
      --diff-weight
      -keep-file
      --keep-file
      -log-level
      --log-level
      -mb-base-url
      --mb-base-url
      -mb-rate-limit
      --mb-rate-limit
      -notification-uri
      --notification-uri
      -path-format
      --path-format
      -research-link
      --research-link
      -tag-config
      --tag-config
      -version
      --version
      move
      copy
      reflink
      sync
    )
  # complete options for operation commands
  elif [[ "${IFS}${PREV_COMP_WORDS[*]}${IFS}" =~ ${IFS}(move|copy|reflink)${IFS} ]]; then
    opts=(
      -h
      -help
      --help
      -dry-run
      --dry-run
      -mbid
      --mbid
      -yes
      --yes
    )
  # complete options for sync commands
  elif [[ "${IFS}${PREV_COMP_WORDS[*]}${IFS}" =~ ${IFS}sync${IFS} ]]; then
    opts=(
      -h
      -help
      --help
      -age-older
      --age-older
      -age-younger
      --age-younger
      -dry-run
      --dry-run
      -num-workers
      --num-workers
    )
  fi

  readarray -d $'\0' COMPREPLY < <(printf "%s\0" "${opts[@]}" | grep -zx -- "${PREV_COMP_WORDS[-1]}.*")
}

complete -F _wrtag_completion wrtag
