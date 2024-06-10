# A simple script to download the relevant files from the 12y markup repo.
# Note that this isn't a submodule, this is done to better control versioning
set -e

# NOTE: there is a chance that 'cactus' won't be the right branch someday
MBASE="https://raw.githubusercontent.com/12Me21/markup2/cactus"
FILES="legacy.js langs.js render.js helpers.js parse.js markup.css"
DESTINATION="static/markup"

cd $DESTINATION
rm -f $FILES

for f in $FILES; do
	wget "$MBASE/$f"
done
