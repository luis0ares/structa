import { useEffect, useState } from 'react';
import { main } from '../../wailsjs/go/models';
import {
  CreateProfile,
  DeleteProfile,
  DetectProfile,
  GetActiveProfile,
  GetProfiles,
  PickFolder,
  SelectProfile,
  UpdateProfile,
} from '../../wailsjs/go/main/App';
import { FolderIcon, PencilIcon, PlusIcon, TrashIcon, UserIcon } from '../components/icons';
import { useConfirm } from '../components/ConfirmDialog';

type Profile = main.ProfileDTO;
type EditState = { name: string; dataDir: string };

export function ProfileView({ onSelected }: { onSelected: () => void }) {
  const [profileList, setProfileList] = useState<Profile[]>([]);
  const [active, setActive] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState('');
  const [newDir, setNewDir] = useState('');
  const [isImport, setIsImport] = useState(false);
  const [editingName, setEditingName] = useState<string | null>(null);
  const [editState, setEditState] = useState<EditState | null>(null);
  const confirm = useConfirm();

  const load = async () => {
    const [profs, act] = await Promise.all([GetProfiles(), GetActiveProfile()]);
    setProfileList(profs ?? []);
    setActive(act ?? '');
  };

  useEffect(() => { load(); }, []);

  const handleSelect = async (name: string) => {
    setError('');
    setBusy(true);
    try {
      await SelectProfile(name);
      onSelected();
    } catch (err) {
      setError(String(err));
      setBusy(false);
    }
  };

  const handleCreate = async () => {
    if (!newName.trim() || !newDir) return;
    setError('');
    setBusy(true);
    try {
      await CreateProfile(newName.trim(), newDir);
      setCreating(false);
      setNewName('');
      setNewDir('');
      await handleSelect(newName.trim());
    } catch (err) {
      setError(String(err));
      setBusy(false);
    }
  };

  const handlePickNewDir = async () => {
    const dir = await PickFolder();
    if (!dir) return;
    const isDotStructa = dir.replace(/\\/g, '/').endsWith('/.structa');
    setIsImport(isDotStructa);
    if (isDotStructa) {
      const detected = await DetectProfile(dir);
      setNewDir(detected.data_dir);
      if (!newName) setNewName(detected.name);
    } else {
      setNewDir(dir);
    }
  };

  const handlePickEditDir = async () => {
    const dir = await PickFolder();
    if (dir && editState) setEditState({ ...editState, dataDir: dir });
  };

  const handleImport = async () => {
    const dir = await PickFolder();
    if (!dir) return;
    const detected = await DetectProfile(dir);
    setNewName(detected.name);
    setNewDir(detected.data_dir);
    setIsImport(dir.replace(/\\/g, '/').endsWith('/.structa'));
    setCreating(true);
  };

  const handleDelete = async (name: string) => {
    const ok = await confirm({
      title: 'Remove profile',
      message: `Remove profile "${name}"? The data folder will NOT be deleted.`,
      confirmLabel: 'Remove',
      cancelLabel: 'Cancel',
      danger: true,
    });
    if (!ok) return;
    await DeleteProfile(name);
    await load();
  };

  const handleUpdate = async () => {
    if (!editState || !editingName) return;
    setError('');
    setBusy(true);
    try {
      await UpdateProfile(editingName, editState.name.trim(), editState.dataDir);
      setEditingName(null);
      setEditState(null);
      await load();
    } catch (err) {
      setError(String(err));
    } finally {
      setBusy(false);
    }
  };

  const cancelCreate = () => { setCreating(false); setNewName(''); setNewDir(''); setIsImport(false); setError(''); };
  const cancelEdit = () => { setEditingName(null); setEditState(null); setError(''); };

  return (
    <div className="profile-screen">
      <div className="profile-panel">
        <div className="profile-header">
          <UserIcon size={32} className="profile-logo-icon" />
          <h1 className="profile-title">Structa</h1>
          <p className="profile-subtitle">
            {profileList.length === 0 ? 'Create your first profile to get started' : 'Select a profile to continue'}
          </p>
        </div>

        {profileList.length > 0 && (
          <div className="profile-list">
            {profileList.map((p) => (
              <div
                key={p.name}
                className={`profile-row ${active === p.name ? 'profile-row-active' : ''}`}
              >
                {editingName === p.name && editState ? (
                  <div className="profile-inline-form">
                    <input
                      className="profile-input"
                      value={editState.name}
                      onChange={(e) => setEditState({ ...editState, name: e.target.value })}
                      placeholder="Profile name"
                      autoFocus
                    />
                    <div className="profile-dir-row">
                      <input
                        className="profile-input"
                        value={editState.dataDir}
                        placeholder="Data folder"
                        readOnly
                        style={{ flex: 1 }}
                      />
                      <button className="profile-btn-icon" onClick={handlePickEditDir} title="Pick folder">
                        <FolderIcon />
                      </button>
                    </div>
                    <div className="profile-row-actions">
                      <button className="profile-btn" onClick={cancelEdit} disabled={busy}>Cancel</button>
                      <button
                        className="profile-btn primary"
                        onClick={handleUpdate}
                        disabled={busy || !editState.name.trim()}
                      >
                        Save
                      </button>
                    </div>
                  </div>
                ) : (
                  <>
                    <div className="profile-info">
                      <div className="profile-name-row">
                        <span className="profile-name">{p.name}</span>
                        {active === p.name && <span className="profile-badge">last used</span>}
                      </div>
                      <span className="profile-dir" title={p.data_dir}>{p.data_dir}</span>
                    </div>
                    <div className="profile-row-actions">
                      <button
                        className="profile-btn-icon"
                        onClick={() => { setEditingName(p.name); setEditState({ name: p.name, dataDir: p.data_dir }); }}
                        title="Edit"
                        disabled={busy}
                      >
                        <PencilIcon size={14} />
                      </button>
                      <button
                        className="profile-btn-icon danger"
                        onClick={() => handleDelete(p.name)}
                        title="Remove"
                        disabled={busy}
                      >
                        <TrashIcon size={14} />
                      </button>
                      <button
                        className="profile-btn primary"
                        onClick={() => handleSelect(p.name)}
                        disabled={busy}
                      >
                        {busy && active === p.name ? 'Opening…' : 'Open'}
                      </button>
                    </div>
                  </>
                )}
              </div>
            ))}
          </div>
        )}

        {creating ? (
          <div className="profile-create-card">
            <p className="profile-create-label">New profile</p>
            <input
              className="profile-input"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder="Profile name"
              autoFocus
              onKeyDown={(e) => { if (e.key === 'Enter') handleCreate(); if (e.key === 'Escape') cancelCreate(); }}
            />
            <div className="profile-dir-row">
              <input
                className="profile-input"
                value={newDir}
                placeholder="Data folder — a .structa folder will be created here"
                readOnly
                style={{ flex: 1 }}
              />
              <button className="profile-btn-icon" onClick={handlePickNewDir} title="Pick folder">
                <FolderIcon />
              </button>
            </div>
            {newDir && (
              <p className="profile-hint">
                {isImport
                  ? <>Reconnecting to existing data in <code>{newDir}</code></>
                  : <>Data will be stored in <code>{newDir}\.structa\</code></>}
              </p>
            )}
            <div className="profile-row-actions">
              <button className="profile-btn" onClick={cancelCreate} disabled={busy}>Cancel</button>
              <button
                className="profile-btn primary"
                onClick={handleCreate}
                disabled={busy || !newName.trim() || !newDir}
              >
                {busy ? 'Creating…' : 'Create & Open'}
              </button>
            </div>
          </div>
        ) : (
          <div className="profile-bottom-actions">
            <button className="profile-btn" onClick={() => setCreating(true)} disabled={busy}>
              <PlusIcon size={14} />
              New Profile
            </button>
            <button className="profile-btn" onClick={handleImport} disabled={busy}>
              <FolderIcon />
              Import from folder
            </button>
          </div>
        )}

        {error && <div className="profile-error">{error}</div>}
      </div>
    </div>
  );
}
