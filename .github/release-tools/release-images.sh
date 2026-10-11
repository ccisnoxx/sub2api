#!/usr/bin/env bash
set -euo pipefail
: "${RELEASE_VERSION:?}" "${RELEASE_SHA:?}" "${RELEASE_DATE:?}" "${GITHUB_REPOSITORY:?}" "${RUNNER_TEMP:?}"
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
scanner=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["sbom_generator"])' "$RUNNER_TEMP/release-tools/supply-chain-tools.json")
verify_image() {
  local layout=$1 arch=$2 stage=$3
  local created_args=()
  if [[ $stage == built ]]; then created_args=(--created "$RELEASE_DATE"); fi
  python3 "$RUNNER_TEMP/release-tools/supply_chain.py" image --layout "$layout" \
    --sha "$RELEASE_SHA" --version "$RELEASE_VERSION" --platforms "linux/$arch" \
    "${created_args[@]}" --output "release-evidence/image-$arch-$stage"
}
for arch in "${arches[@]}"; do
  oci="$RUNNER_TEMP/sub2api-$arch.oci.tar"
  image_name="${registries[0]}:$RELEASE_VERSION-$arch"
  if [[ ${SIMPLE_RELEASE:-false} == true ]]; then image_name="${registries[0]}:$RELEASE_VERSION"; fi
  if [[ $personal != true || ${REUSE_IMAGE:-false} != true ]]; then
    # OCI 导出保留证明；docker --load 会丢失默认 image store 不支持的证明。
    docker buildx build --tag "$image_name" --platform "linux/$arch" --file ".release-context/$arch/Dockerfile" \
      --label "org.opencontainers.image.version=$RELEASE_VERSION" \
      --label "org.opencontainers.image.revision=$RELEASE_SHA" \
      --label "org.opencontainers.image.source=https://github.com/$GITHUB_REPOSITORY" \
      --label "org.opencontainers.image.created=$RELEASE_DATE" \
      --attest "type=sbom,generator=$scanner" --attest type=provenance,mode=max \
      --output "type=oci,dest=$oci" ".release-context/$arch"
    digest=$(verify_image "$oci" "$arch" built)
  fi
  if [[ ${DRY_RUN:-false} == true ]]; then continue; fi
  if [[ $personal == true ]]; then
    # 实际推送前重新核对最新 personal、CI 和版本占用；已有版本只校验，不重建。
    python3 "$RUNNER_TEMP/release-tools/personal_release.py" authorize \
      --sha "$RELEASE_SHA" --tag "$RELEASE_TAG" --ci-run-id "${CI_RUN_ID:-}" > "$RUNNER_TEMP/push-guard.json"
    reuse=$(python3 -c 'import json,sys; print(str(json.load(open(sys.argv[1]))["reuse_image"]).lower())' "$RUNNER_TEMP/push-guard.json")
    if [[ $reuse != true ]]; then
      oras cp --from-oci-layout "$oci@$digest" "${registries[0]}:$RELEASE_VERSION"
    fi
    python3 "$RUNNER_TEMP/release-tools/personal_release.py" verify-image \
      --sha "$RELEASE_SHA" --tag "$RELEASE_TAG" > "$RUNNER_TEMP/pushed-image.json"
    published=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["digest"])' "$RUNNER_TEMP/pushed-image.json")
    if [[ $reuse != true && $published != "$digest" ]]; then echo '发布 digest 与已校验 OCI 产物不同'; exit 1; fi
    oras cp --to-oci-layout "${registries[0]}@$published" "$RUNNER_TEMP/verify-$arch:verified"
    verified=$(verify_image "$RUNNER_TEMP/verify-$arch" "$arch" published)
    [[ $verified == "$published" ]] || { echo '远端 OCI digest 不匹配'; exit 1; }
    # latest 仅从已经完整核对的固定 digest 补齐，证明缺失时不推进。
    oras tag "${registries[0]}@$published" latest
    continue
  fi
  for registry in "${registries[@]}"; do
    tag="$RELEASE_VERSION-$arch"
    if [[ ${SIMPLE_RELEASE:-false} == true ]]; then tag="$RELEASE_VERSION"; fi
    oras cp --from-oci-layout "$oci@$digest" "$registry:$tag"
    oras cp --to-oci-layout "$registry@$digest" "$RUNNER_TEMP/verify-$arch:verified"
    verified=$(verify_image "$RUNNER_TEMP/verify-$arch" "$arch" published)
    [[ $verified == "$digest" ]] || { echo '远端 OCI digest 不匹配'; exit 1; }
    if [[ ${SIMPLE_RELEASE:-false} == true ]]; then oras tag "$registry@$digest" latest; fi
  done
done
if [[ ${DRY_RUN:-false} != true && ${SIMPLE_RELEASE:-false} != true ]]; then
  major=${RELEASE_VERSION%%.*}
  minor=${RELEASE_VERSION#*.}; minor=${minor%%.*}
  for registry in "${registries[@]}"; do
    docker buildx imagetools create --tag "$registry:$RELEASE_VERSION" \
      "$registry:$RELEASE_VERSION-amd64" "$registry:$RELEASE_VERSION-arm64"
    digest=$(oras resolve "$registry:$RELEASE_VERSION")
    oras cp --to-oci-layout "$registry@$digest" "$RUNNER_TEMP/verify-matrix:verified"
    verified=$(python3 "$RUNNER_TEMP/release-tools/supply_chain.py" image \
      --layout "$RUNNER_TEMP/verify-matrix" --sha "$RELEASE_SHA" --version "$RELEASE_VERSION" \
      --platforms linux/amd64 linux/arm64 --output release-evidence/image-matrix-published)
    [[ $verified == "$digest" ]] || { echo '多平台发布 digest 不匹配'; exit 1; }
    oras tag "$registry@$digest" latest "$major.$minor" "$major"
  done
fi
