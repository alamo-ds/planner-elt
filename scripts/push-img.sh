if [ "$#" -eq 0 ]; then
    tag="$(git describe --tags --abbrev=0)"
else
    tag="${1}"
fi
echo "pushing image with tag: ${tag}"

docker push alamods/planner-elt:"${tag}"
