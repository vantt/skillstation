import { Link } from 'react-router';
import { useCheckouts } from '../../api/queries';
import { useT } from '../../i18n';
import { checkoutDate } from '../checkouts/checkout-format';

interface OpenCheckoutsNoteProps {
  skillId: string;
}

// One line per project that has this skill checked out, linking to the Checkouts page.
// Shows nothing while the list loads or when it cannot be read.
export function OpenCheckoutsNote({ skillId }: OpenCheckoutsNoteProps) {
  const t = useT();
  const { data } = useCheckouts();
  const open = (data?.checkouts ?? []).filter((checkout) => checkout.skill_id === skillId && checkout.state === 'open');
  if (open.length === 0) return null;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-1)' }}>
      {open.map((checkout) => (
        <Link
          key={checkout.project_root}
          to="/checkouts"
          className="t-body-sm"
          style={{ color: 'var(--color-link)', textDecoration: 'none', overflowWrap: 'anywhere' }}
        >
          <span>
            {t('checkouts.open_in', { project: checkout.project_root, date: checkoutDate(checkout.started_at) })}
          </span>
        </Link>
      ))}
    </div>
  );
}
