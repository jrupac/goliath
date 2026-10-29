import React from 'react';
import { Dialog, DialogTitle, DialogContent, Typography } from '@mui/material';
import { Keybindings, Keybinding } from '../utils/keybindings';

interface KeybindingsModalProps {
  open: boolean;
  onClose: () => void;
}

const KeybindingsModal: React.FC<KeybindingsModalProps> = ({
  open,
  onClose,
}) => {
  return (
    <Dialog
      open={open}
      onClose={onClose}
      maxWidth="sm"
      fullWidth
      slotProps={{
        backdrop: { className: 'GoliathModalOverlay' },
        paper: { className: 'GoliathOverlayPaper' },
      }}
    >
      <DialogTitle className="GoliathDialogTitle">
        Keyboard Shortcuts
      </DialogTitle>
      <DialogContent className="GoliathDialogContent">
        <KeybindingSection title="Global" bindings={Keybindings.global} />
        <KeybindingSection
          title="Article List"
          bindings={Keybindings.articleList}
        />
        <KeybindingSection
          title="Article View"
          bindings={Keybindings.articleView}
        />
      </DialogContent>
    </Dialog>
  );
};

interface KeybindingSectionProps {
  title: string;
  bindings: Keybinding[];
}

const KeybindingSection: React.FC<KeybindingSectionProps> = ({
  title,
  bindings,
}) => {
  if (bindings.length === 0) return null;

  return (
    <div className="GoliathKeybindingSection">
      <Typography variant="subtitle2" className="GoliathKeybindingSectionTitle">
        {title}
      </Typography>
      {bindings
        .filter((kb) => kb.display.length > 0)
        .map((kb, idx) => (
          <div key={kb.handlerKey + idx} className="GoliathKeybindingRow">
            <div className="GoliathKeybindingKeyGroup">
              {kb.display.map((chip, chipIdx) => (
                <React.Fragment key={chipIdx}>
                  {chipIdx > 0 && (
                    <span className="GoliathKeybindingCombo">
                      {kb.isSequential ? '⭢' : kb.isChord ? '+' : '/'}
                    </span>
                  )}
                  <div className="GoliathKeybindingKey">{chip}</div>
                </React.Fragment>
              ))}
            </div>
            <Typography className="GoliathKeybindingLabel">
              {kb.label}
            </Typography>
          </div>
        ))}
    </div>
  );
};

export default KeybindingsModal;
