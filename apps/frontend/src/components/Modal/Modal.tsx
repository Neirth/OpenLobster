// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component, JSX, ParentProps } from "solid-js";
import { Show } from "solid-js";
import { t } from "../../App";
import "./Modal.css";

export interface ModalProps extends ParentProps {
  isOpen: boolean;
  onClose: () => void;
  title: string;
  children: JSX.Element;
}

const Modal: Component<ModalProps> = (props) => {
  const handleOverlayClick = (e: MouseEvent) => {
    if (e.target === e.currentTarget) {
      props.onClose();
    }
  };

  return (
    <Show when={props.isOpen}>
      <div class="modal-overlay" onClick={handleOverlayClick}>
        <div class="modal-box">
          <div class="modal-header">
            <h3 class="modal-title">{props.title}</h3>
            <button
              class="modal-close"
              onClick={() => props.onClose()}
              aria-label={t("common.closeAria")}
            >
              <span class="material-symbols-outlined">close</span>
            </button>
          </div>
          <div class="modal-content">{props.children}</div>
        </div>
      </div>
    </Show>
  );
};

export default Modal;
