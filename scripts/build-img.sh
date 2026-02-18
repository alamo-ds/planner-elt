if [ "$#" -eq 0 ]; then
    tag="$(git describe --tags --abbrev=0)"
else
    tag="${1}"
fi
echo "building for tag: ${tag}"

docker buildx build -t alamods/planner-elt:"${tag}" --provenance=true --sbom=true .
