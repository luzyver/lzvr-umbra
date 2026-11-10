// adapter-static empties the output folder; .keep is recreated so go:embed still works in a fresh clone.
import { writeFileSync } from 'node:fs';
writeFileSync(new URL('../../internal/ui/dist/.keep', import.meta.url), '');
