# Notice

## Quran Text Attribution And Terms

This project uses Quran text data from the Tanzil Project.

Source:

- Tanzil Quran Text: https://tanzil.net/
- Text license page: https://tanzil.net/docs/Text_License
- Updates feed reference: https://tanzil.net/updates/

Attribution details (as published by Tanzil):

- Copyright holder: Tanzil Project
- License family: Creative Commons Attribution 3.0
- Distribution rule: verbatim copying/distribution is allowed
- Integrity rule: changing Quran text content is not allowed
- Usage rule: applications must clearly indicate Tanzil as source and link to `tanzil.net`
- Notice rule: this attribution notice must be preserved in copies/derived files containing substantial Quran text

Repository policy derived from the above:

- `data/quran.json` must preserve source Quran text without content edits
- contributors must retain this notice when updating or redistributing dataset files
- dataset update PRs should confirm source and date/version used

## Translation Attribution And Terms

This project also bundles translations downloaded from:

- English (`en`): Quran.com verse-route Next data payload (default English translation currently shown as Dr. Mustafa Khattab, The Clear Quran)
- Quran.com website: https://quran.com
- Quran Foundation API docs: https://api-docs.quran.com

Indonesian translation and tafsir data is imported from:

- Repository: https://github.com/rioastamal/quran-json
- Source basis noted by repository author: https://quran.kemenag.go.id
- License (repository): MIT

English tafsir data is imported from Quran Foundation API endpoint:

- endpoint pattern: `https://api.quran.com/api/v4/tafsirs/{id}/by_chapter/{chapter}`
- current resource id: `169` (Ibn Kathir (Abridged))

## Project Code License

Project source code is licensed under the MIT License (see `LICENSE`).
