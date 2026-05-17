import React from 'react';
import {
  Dialog,
  DialogTitle,
  DialogContent,
  Box,
  Typography,
} from '@mui/material';
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
        backdrop: { className: 'GoliathKeybindingsModalOverlay' },
      }}
      PaperProps={{
        className: 'GoliathKeybindingsModalPaper',
      }}
    >
      <DialogTitle className="GoliathKeybindingsDialogTitle">
        Keyboard Shortcuts
      </DialogTitle>
      <DialogContent className="GoliathKeybindingsDialogContent">
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
    <Box className="GoliathKeybindingSection">
      <Typography variant="subtitle2" className="GoliathKeybindingSectionTitle">
        {title}
      </Typography>
      {bindings
        .filter((kb) => kb.display.length > 0)
        .map((kb, idx) => (
          <Box key={kb.handlerKey + idx} className="GoliathKeybindingRow">
            <Box className="GoliathKeybindingKeyGroup">
              {kb.display.map((chip, chipIdx) => (
                <React.Fragment key={chipIdx}>
                  {chipIdx > 0 && (
                    <span className="GoliathKeybindingCombo">
                      {kb.isSequential ? '⭢' : kb.isChord ? '+' : '/'}
                    </span>
                  )}
                  <Box className="GoliathKeybindingKey">{chip}</Box>
                </React.Fragment>
              ))}
            </Box>
            <Typography className="GoliathKeybindingLabel">
              {kb.label}
            </Typography>
          </Box>
        ))}
    </Box>
  );
};

export default KeybindingsModal;
