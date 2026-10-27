// adapter-static mengosongkan folder output; .keep dibuat ulang supaya go:embed tetap jalan di clone baru.
import { writeFileSync } from 'node:fs';
writeFileSync(new URL('../../internal/ui/dist/.keep', import.meta.url), '');
