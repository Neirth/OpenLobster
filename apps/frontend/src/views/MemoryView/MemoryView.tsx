// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * MemoryView — knowledge graph browser.
 *
 * Left panel: node list grouped by type.
 * Right panel: detail view for the selected node.
 * Empty state shown when no node is selected.
 *
 * Section headings are derived dynamically from the distinct `type` values
 * present in the memory graph — no type names are hardcoded here.
 */

import type { Component } from 'solid-js';
import { For, Index, createMemo, createSignal, Show } from 'solid-js';
import { createMutation, useQueryClient } from '@tanstack/solid-query';
import { useMemory } from '@openlobster/ui/hooks';
import type { MemoryNode } from '@openlobster/ui/types';
import { UPDATE_MEMORY_NODE_MUTATION, DELETE_MEMORY_NODE_MUTATION } from '@openlobster/ui/graphql/mutations';
import { client } from '../../graphql/client';
import AppShell from '../../components/AppShell';
import Modal from '../../components/Modal';
import { t } from '../../App';
import GraphVisualization from '../../components/GraphVisualization';
import './MemoryView.css';

/**
 * Groups an array of MemoryNode objects by their `type` field.
 *
 * @param nodes - Flat list of memory nodes from the graph.
 * @returns A Map where each key is a distinct node type and the value
 *          is the ordered list of nodes belonging to that type.
 */
function groupNodesByType(nodes: MemoryNode[]): Map<string, MemoryNode[]> {
  const groups = new Map<string, MemoryNode[]>();
  for (const node of nodes) {
    const key = node.type ?? '';
    const existing = groups.get(key);
    if (existing) {
      existing.push(node);
    } else {
      groups.set(key, [node]);
    }
  }
  return groups;
}

const MemoryView: Component = () => {
  const memory = useMemory(client);
  const queryClient = useQueryClient();
  const [selectedNode, setSelectedNode] = createSignal<MemoryNode | null>(null);

  // Edit modal state
  const [editModalOpen, setEditModalOpen] = createSignal(false);
  const [editLabel, setEditLabel] = createSignal('');
  const [editType, setEditType] = createSignal('');
  const [editProperties, setEditProperties] = createSignal<Array<{ key: string; value: string }>>([]);

  // Delete modal state
  const [deleteModalOpen, setDeleteModalOpen] = createSignal(false);

  // Sidebar search
  const [searchQuery, setSearchQuery] = createSignal('');

  const updateNode = createMutation(() => ({
    mutationFn: (vars: { id: string; label: string; type: string; value: string; properties: string }) =>
      client.request(UPDATE_MEMORY_NODE_MUTATION, vars),
    onSuccess: (data: { updateMemoryNode: MemoryNode }) => {
      void queryClient.invalidateQueries({ queryKey: ['memory'] });
      // Merge updated properties back since the resolver returns them
      const updated = data.updateMemoryNode;
      const propsMap: Record<string, string> = {};
      for (const { key, value } of editProperties()) {
        if (key.trim()) propsMap[key.trim()] = value;
      }
      setSelectedNode({ ...updated, properties: propsMap });
      setEditModalOpen(false);
    },
  }));

  const deleteNode = createMutation(() => ({
    mutationFn: (vars: { id: string }) => client.request(DELETE_MEMORY_NODE_MUTATION, vars),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['memory'] });
      setSelectedNode(null);
      setDeleteModalOpen(false);
    },
  }));

  const openEditModal = () => {
    const node = selectedNode();
    if (!node) return;
    setEditLabel(node.label);
    setEditType(node.type);
    const props = node.properties ? Object.entries(node.properties).map(([k, v]) => ({ key: k, value: v })) : [];
    setEditProperties(props);
    setEditModalOpen(true);
  };

  const addProperty = () =>
    setEditProperties((p) => [...p, { key: '', value: '' }]);

  const removeProperty = (index: number) =>
    setEditProperties((p) => p.filter((_, i) => i !== index));

  const updatePropertyKey = (index: number, key: string) =>
    setEditProperties((p) => p.map((item, i) => (i === index ? { ...item, key } : item)));

  const updatePropertyValue = (index: number, value: string) =>
    setEditProperties((p) => p.map((item, i) => (i === index ? { ...item, value } : item)));

  const handleEditSubmit = (e: Event) => {
    e.preventDefault();
    const node = selectedNode();
    if (!node) return;
    const propsMap: Record<string, string> = {};
    for (const { key, value } of editProperties()) {
      if (key.trim()) propsMap[key.trim()] = value;
    }
    updateNode.mutate({
      id: node.id,
      label: editLabel(),
      type: editType(),
      value: node.value ?? '',
      properties: JSON.stringify(propsMap),
    });
  };

  const handleDeleteConfirm = () => {
    const node = selectedNode();
    if (!node) return;
    deleteNode.mutate({ id: node.id });
  };

  const groupedNodes = createMemo(() => {
    const q = searchQuery().trim().toLowerCase();
    const nodes = memory.data?.nodes ?? [];
    const filtered = q
      ? nodes.filter(
          (n) =>
            (n.label ?? '').toLowerCase().includes(q) ||
            (n.type ?? '').toLowerCase().includes(q) ||
            (n.value ?? '').toLowerCase().includes(q)
        )
      : nodes;
    const map = groupNodesByType(filtered);
    // Sort groups alphabetically, and nodes within each group alphabetically
    return new Map(
      [...map.entries()]
        .sort(([a], [b]) => a.localeCompare(b))
        .map(([type, items]) => [
          type,
          [...items].sort((a, b) => (a.label ?? '').localeCompare(b.label ?? '')),
        ])
    );
  });

  // Get all edges connected to the selected node (memoized)
  const selectedNodeEdges = createMemo(() => {
    const node = selectedNode();
    if (!node || !memory.data?.edges) return { outgoing: [], incoming: [] };
    
    const outgoing = memory.data.edges.filter(e => e.sourceId === node.id);
    const incoming = memory.data.edges.filter(e => e.targetId === node.id);
    
    return { outgoing, incoming };
  });

  // Find node by ID
  const findNodeById = (id: string): MemoryNode | undefined => {
    return memory.data?.nodes.find(n => n.id === id);
  };

  const hasNodes = () => (memory.data?.nodes?.length ?? 0) > 0;

  return (
    <AppShell activeTab="memory" fullHeight>
      <div class="memory-view">
        <Show when={!memory.isLoading && !hasNodes()}>
          <div class="memory-empty">
            <span class="material-symbols-outlined memory-empty__icon">psychology</span>
            <p class="memory-empty__title">{t('memory.noMemory')}</p>
            <p class="memory-empty__hint">{t('memory.noMemoryHint')}</p>
          </div>
        </Show>

        <Show when={hasNodes()}>
        <div class="memory-container">
          {/* Sidebar */}
          <aside class="memory-sidebar">
            <div class="sidebar-header">
              <h2>{t('memory.memoryIndex')}</h2>
              <input
                type="text"
                class="search-box"
                placeholder={t('memory.searchPlaceholder')}
                value={searchQuery()}
                onInput={(e) => setSearchQuery(e.currentTarget.value)}
              />
            </div>

            <For each={[...groupedNodes().entries()]}>
              {([type, nodes]) => (
                <div class="memory-section">
                  <h3>{type.replace(/_/g, ' ').toUpperCase()}</h3>
                  <ul class="memory-list">
                    <For each={nodes}>
                      {(node) => (
                        <li class="memory-item" classList={{ 'memory-item--active': selectedNode()?.id === node.id }} onClick={() => setSelectedNode(node)}>
                          <div class="memory-item-avatar">
                            <span class="avatar-placeholder">{(node.label ?? '?').charAt(0)}</span>
                          </div>
                          <div class="memory-item-info">
                            <span class="memory-item-name">{node.label ?? ''}</span>
                            <span class="memory-item-role">{node.type ?? ''}</span>
                          </div>
                        </li>
                      )}
                    </For>
                  </ul>
                </div>
              )}
            </For>
          </aside>

          {/* Main Content */}
          <main class="memory-content">
            {selectedNode() ? (
              <div class="person-detail">
                <div class="person-header">
                  <div class="person-avatar">
                    <span class="avatar-large">{(selectedNode()!.label ?? '?').charAt(0)}</span>
                  </div>
                  <div class="person-info">
                    <h1>{selectedNode()!.label ?? ''}</h1>
                    <span class="person-role">{selectedNode()!.type ?? ''}</span>
                  </div>
                  <div class="person-actions">
                    <button class="action-btn" onClick={openEditModal} title={t('memory.editNode')}>
                      <span class="material-symbols-outlined">edit</span>
                    </button>
                    <button class="action-btn action-btn--danger" onClick={() => setDeleteModalOpen(true)} title={t('memory.deleteNode')}>
                      <span class="material-symbols-outlined">delete</span>
                    </button>
                  </div>
                </div>

                <Show when={selectedNode()!.properties && Object.keys(selectedNode()!.properties!).length > 0}>
                  <section class="detail-section">
                    <h2>{t('memory.properties')}</h2>
                    <div class="properties-list">
                      <For each={Object.entries(selectedNode()!.properties!)}>
                        {([key, value]) => (
                          <div class="property-item">
                            <span class="property-key">{key}:</span>
                            <span class="property-value">{value}</span>
                          </div>
                        )}
                      </For>
                    </div>
                  </section>
                </Show>

                <section class="detail-section">
                  <h2>{t('memory.graphViz')}</h2>
                  <GraphVisualization 
                    nodes={memory.data?.nodes ?? []}
                    edges={memory.data?.edges ?? []}
                    selectedNodeId={selectedNode()?.id}
                    onNodeSelect={setSelectedNode}
                  />
                </section>

                <section class="detail-section">
                  <h2>{t('memory.connections')}</h2>
                  <Show when={selectedNodeEdges().outgoing.length > 0} fallback={<p class="no-connections">{t('memory.noOutgoing')}</p>}>
                    <div class="connections-list">
                      <h3>{t('memory.outgoing')} ({selectedNodeEdges().outgoing.length})</h3>
                      <ul class="edge-list">
                        <For each={selectedNodeEdges().outgoing}>
                          {(edge) => {
                            const targetNode = findNodeById(edge.targetId);
                            return (
                              <li class="edge-item" onClick={() => targetNode && setSelectedNode(targetNode)}>
                                <span class="edge-relation">{edge.relation ?? ''}</span>
                                <span class="edge-arrow">→</span>
                                <span class="edge-target">{targetNode?.label ?? edge.targetId}</span>
                                <span class="edge-type">({targetNode?.type ?? t('memory.unknown')})</span>
                              </li>
                            );
                          }}
                        </For>
                      </ul>
                    </div>
                  </Show>
                  
                  <Show when={selectedNodeEdges().incoming.length > 0}>
                    <div class="connections-list">
                      <h3>{t('memory.incoming')} ({selectedNodeEdges().incoming.length})</h3>
                      <ul class="edge-list">
                        <For each={selectedNodeEdges().incoming}>
                          {(edge) => {
                            const sourceNode = findNodeById(edge.sourceId);
                            return (
                              <li class="edge-item" onClick={() => sourceNode && setSelectedNode(sourceNode)}>
                                <span class="edge-source">{sourceNode?.label ?? edge.sourceId}</span>
                                <span class="edge-type">({sourceNode?.type ?? t('memory.unknown')})</span>
                                <span class="edge-arrow">→</span>
                                <span class="edge-relation">{edge.relation ?? ''}</span>
                              </li>
                            );
                          }}
                        </For>
                      </ul>
                    </div>
                  </Show>
                </section>
              </div>
            ) : (
              <div class="no-selection">
                <span class="material-symbols-outlined">memory</span>
                <p>{t('memory.selectToView')}</p>
              </div>
            )}
          </main>
        </div>
        </Show>
      </div>
      {/* Edit Modal */}
      <Modal isOpen={editModalOpen()} onClose={() => setEditModalOpen(false)} title={t('memory.editNode')}>
        <form class="memory-modal-form" onSubmit={handleEditSubmit}>
          <div class="memory-modal-field">
            <label for="edit-label">{t('memory.label')}</label>
            <input
              id="edit-label"
              type="text"
              value={editLabel()}
              onInput={(e) => setEditLabel(e.currentTarget.value)}
              required
            />
          </div>
          <div class="memory-modal-field">
            <label for="edit-type">{t('memory.type')}</label>
            <select
              id="edit-type"
              value={editType()}
              onChange={(e) => setEditType(e.currentTarget.value)}
              required
            >
              <option value="fact">fact</option>
              <option value="person">person</option>
              <option value="place">place</option>
              <option value="organization">organization</option>
              <option value="event">event</option>
              <option value="thing">thing</option>
              <option value="story">story</option>
            </select>
          </div>

          <div class="memory-modal-properties">
            <div class="memory-modal-properties-header">
              <span class="memory-modal-properties-label">{t('memory.properties')}</span>
              <button type="button" class="memory-modal-add-prop" onClick={addProperty}>
                <span class="material-symbols-outlined">add</span>
                {t('memory.addProperty')}
              </button>
            </div>
            <Show when={editProperties().length > 0}>
              <div class="memory-modal-props-list">
                <Index each={editProperties()}>
                  {(prop, i) => (
                    <div class="memory-modal-prop-row">
                      <input
                        type="text"
                        placeholder={t('memory.key')}
                        value={prop().key}
                        onInput={(e) => updatePropertyKey(i, e.currentTarget.value)}
                      />
                      <span class="prop-row-sep">:</span>
                      <input
                        type="text"
                        placeholder={t('memory.value')}
                        value={prop().value}
                        onInput={(e) => updatePropertyValue(i, e.currentTarget.value)}
                      />
                      <button
                        type="button"
                        class="prop-row-remove"
                        onClick={() => removeProperty(i)}
                        title={t('memory.remove')}
                      >
                        <span class="material-symbols-outlined">close</span>
                      </button>
                    </div>
                  )}
                </Index>
              </div>
            </Show>
            <Show when={editProperties().length === 0}>
              <p class="memory-modal-props-empty">{t('memory.noProperties')}</p>
            </Show>
          </div>

          <div class="memory-modal-actions">
            <button type="button" class="modal-btn modal-btn--secondary" onClick={() => setEditModalOpen(false)}>
              {t('common.cancel')}
            </button>
            <button type="submit" class="modal-btn modal-btn--primary" disabled={updateNode.isPending}>
              {updateNode.isPending ? t('memory.saving') : t('common.save')}
            </button>
          </div>
        </form>
      </Modal>

      {/* Delete Confirmation Modal */}
      <Modal isOpen={deleteModalOpen()} onClose={() => setDeleteModalOpen(false)} title={t('memory.deleteNode')}>
        <div class="memory-modal-confirm">
          <p>{t('memory.deleteConfirm', { name: selectedNode()?.label || t('memory.unknown') })}</p>
          <p class="memory-modal-warning">{t('memory.deleteWarning')}</p>
          <div class="memory-modal-actions">
            <button type="button" class="modal-btn modal-btn--secondary" onClick={() => setDeleteModalOpen(false)}>
              {t('common.cancel')}
            </button>
            <button
              type="button"
              class="modal-btn modal-btn--danger"
              onClick={handleDeleteConfirm}
              disabled={deleteNode.isPending}
            >
              {deleteNode.isPending ? t('memory.deleting') : t('common.delete')}
            </button>
          </div>
        </div>
      </Modal>
    </AppShell>
  );
};

export default MemoryView;
