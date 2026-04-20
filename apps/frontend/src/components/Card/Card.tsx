// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component, JSX } from 'solid-js';
import './Card.css';

interface CardProps {
  children: JSX.Element;
  class?: string;
  title?: string;
}

const Card: Component<CardProps> = (props) => {
  return (
    <div class={`card ${props.class || ''}`}>
      {props.title && <div class="card-title">{props.title}</div>}
      <div class="card-content">{props.children}</div>
    </div>
  );
};

export { Card };
