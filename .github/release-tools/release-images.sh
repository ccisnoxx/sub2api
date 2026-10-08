#!/usr/bin/env bash
set -euo pipefail
: "${RELEASE_VERSION:?}" "${RELEASE_SHA:?}" "${GITHUB_REPOSITORY:?}" "${RUNNER_TEMP:?}"
owner=${GITHUB_REPOSITORY%%/*}
registries=("ghcr.io/${owner,,}/sub2api")
if [[ ${SIMPLE_RELEASE:-false} != true && ${DOCKERHUB_USERNAME:-skip} != skip ]]; then
  registries+=("${DOCKERHUB_USERNAME}/sub2api")
fi
personal=false
if [[ $GITHUB_REPOSITORY == ccisnoxx/sub2api && ${SIMPLE_RELEASE:-false} == true && ${DRY_RUN:-false} != true ]]; then
  personal=true
fi
arches=(amd64 arm64)
if [[ ${SIMPLE_RELEASE:-false} == true ]]; then arches=(amd64); fi
for arch in "${arches[@]}"; do
  args=(--platform "linux/$arch" --file ".release-context/$arch/Dockerfile"
    --label "org.opencontainers.image.version=$RELEASE_VERSION"
    --label "org.opencontainers.image.revision=$RELEASE_SHA"
    --label "org.opencontainers.image.source=https://github.com/$GITHUB_REPOSITORY")
  if [[ $personal == true ]]; then
    # 先完成本地构建，再在实际推送前核对最新 personal、CI 和版本占用。
    # 个人版本只保留 canonical 标签，避免多标签逐次推送造成部分覆盖。
    args+=(--tag "${registries[0]}:$RELEASE_VERSION" --load)
    if [[ ${REUSE_IMAGE:-false} != true ]]; then
      docker buildx build "${args[@]}" ".release-context/$arch"
    fi
    python3 "$RUNNER_TEMP/release-tools/personal_release.py" authorize \
      --sha "$RELEASE_SHA" --tag "$RELEASE_TAG" --ci-run-id "${CI_RUN_ID:-}" > "$RUNNER_TEMP/push-guard.json"
    reuse=$(python3 -c 'import json,sys; print(str(json.load(open(sys.argv[1]))["reuse_image"]).lower())' "$RUNNER_TEMP/push-guard.json")
    if [[ $reuse != true ]]; then docker push "${registries[0]}:$RELEASE_VERSION"; fi
    python3 "$RUNNER_TEMP/release-tools/personal_release.py" verify-image \
      --sha "$RELEASE_SHA" --tag "$RELEASE_TAG" > "$RUNNER_TEMP/pushed-image.json"
    digest=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["digest"])' "$RUNNER_TEMP/pushed-image.json")
    # latest 是可移动入口；失败恢复仍从已经核验的固定 digest 补齐它。
    docker buildx imagetools create --tag "${registries[0]}:latest" "${registries[0]}@$digest"
    continue
  fi
  for registry in "${registries[@]}"; do
    args+=(--tag "$registry:$RELEASE_VERSION-$arch")
    if [[ ${SIMPLE_RELEASE:-false} == true ]]; then
      args+=(--tag "$registry:$RELEASE_VERSION" --tag "$registry:latest")
    fi
  done
  if [[ ${DRY_RUN:-false} == true ]]; then
    args+=(--output "type=oci,dest=$RUNNER_TEMP/sub2api-$arch.oci.tar")
  else
    args+=(--push)
  fi
  docker buildx build "${args[@]}" ".release-context/$arch"
done
if [[ ${DRY_RUN:-false} != true && ${SIMPLE_RELEASE:-false} != true ]]; then
  major=${RELEASE_VERSION%%.*}
  minor=${RELEASE_VERSION#*.}; minor=${minor%%.*}
  for registry in "${registries[@]}"; do
    docker buildx imagetools create \
      --tag "$registry:$RELEASE_VERSION" --tag "$registry:latest" \
      --tag "$registry:$major.$minor" --tag "$registry:$major" \
      "$registry:$RELEASE_VERSION-amd64" "$registry:$RELEASE_VERSION-arm64"
  done
fi
