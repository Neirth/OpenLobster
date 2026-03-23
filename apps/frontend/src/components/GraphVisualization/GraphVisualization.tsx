/*
 * Copyright (c) 2026 Neirth Intellectual Services
 *
 * This file is part of OpenLobster and is licensed under the MIT License.
 * See the LICENSE file in the project root for license information.
 */

import { onMount, onCleanup, createEffect, createSignal, Show } from 'solid-js';
import type { Core } from 'cytoscape';
import type { MemoryNode, MemoryEdge } from '@openlobster/ui';
import './GraphVisualization.css';

interface GraphVisualizationProps {
  nodes: MemoryNode[];
  edges: MemoryEdge[];
  selectedNodeId?: string;
  onNodeSelect?: (node: MemoryNode | null) => void;
}

export default function GraphVisualization(props: GraphVisualizationProps) {
  let containerRef: HTMLDivElement | undefined;
  let cy: Core | undefined;
  let lastNodeCount = 0;
  let lastEdgeCount = 0;
  const [cytoscapeLoaded, setCytoscapeLoaded] = createSignal(false);

  onMount(async () => {
    if (!containerRef) return;

    const cytoscape = (await import('cytoscape')).default;
    setCytoscapeLoaded(true);

    // Initialize Cytoscape
    cy = cytoscape({
      container: containerRef,
      
      style: [
        {
          selector: 'node',
          style: {
            'background-color': '#3a3a3a',
            'label': 'data(label)',
            'font-family': 'Inter, system-ui, sans-serif',
            'font-size': '10px',
            'font-weight': 400 as unknown as import('cytoscape').Css.FontWeight,
            'text-valign': 'bottom',
            'text-halign': 'center',
            'text-margin-y': 6,
            'color': '#666666',
            'text-outline-width': 0,
            'width': 5,
            'height': 5,
            'border-width': 0,
            'border-opacity': 0,
            'text-opacity': 0,
          }
        },
        {
          selector: 'node:selected',
          style: {
            'background-color': '#5a8ac0',
            'text-outline-width': 0,
            'border-width': 0,
            'border-opacity': 0,
            'width': 8,
            'height': 8,
            'color': '#999999',
            'font-weight': 500 as unknown as import('cytoscape').Css.FontWeight,
            'text-opacity': 1,
          }
        },
        {
          selector: 'node.connected',
          style: {
            'background-color': '#555555',
            'text-outline-width': 0,
            'border-width': 0,
            'border-opacity': 0,
            'text-opacity': 0,
            'width': 4,
            'height': 4,
          }
        },
        {
          selector: 'node.show-label',
          style: {
            'text-opacity': 1,
          }
        },
        {
          selector: 'edge',
          style: {
            'width': 0.5,
            'line-color': '#2a2a2a',
            'target-arrow-color': '#2a2a2a',
            'target-arrow-shape': 'triangle',
            'arrow-scale': 0.3,
            'curve-style': 'bezier',
            'line-style': 'dashed',
            'line-dash-pattern': [4, 3],
            'label': '',
            'opacity': 0.35,
          }
        },
        {
          selector: 'edge.relevant',
          style: {
            'line-color': '#4a7ab5',
            'target-arrow-color': '#4a7ab5',
            'width': 0.75,
            'opacity': 0.5,
            'arrow-scale': 0.4,
            'line-style': 'dashed',
            'line-dash-pattern': [4, 3],
            'label': '',
            'font-family': 'Inter, system-ui, sans-serif',
            'font-size': '10px',
            'font-weight': 500 as unknown as import('cytoscape').Css.FontWeight,
            'color': '#a0a0a0',
            'text-outline-width': 0,
            'text-rotation': 'autorotate',
            'text-margin-y': -8,
            'text-background-color': '#141414',
            'text-background-opacity': 0.9,
            'text-background-padding': '3px',
            'text-background-shape': 'roundrectangle',
          }
        },
        {
          selector: 'edge.dimmed',
          style: {
            'line-color': '#404040',
            'target-arrow-color': '#404040',
            'width': 0.5,
            'opacity': 0.25,
          }
        }
      ],

      layout: {
        name: 'cose',
        animate: false,
        nodeRepulsion: 4000,
        nodeOverlap: 20,
        idealEdgeLength: 40,
        edgeElasticity: 80,
        nestingFactor: 1.2,
        gravity: 1.5,
        numIter: 1000,
        initialTemp: 200,
        coolingFactor: 0.95,
        minTemp: 1.0,
      },

      minZoom: 0.3,
      maxZoom: 5,
    });

    // Handle node selection
    // eslint-disable-next-line solid/reactivity
    cy.on('tap', 'node', (event) => {
      const node = event.target;
      const nodeData = node.data() as { id: string };

      const onNodeSelect = props.onNodeSelect;
      if (onNodeSelect) {
        // Look up the full node from props to preserve all fields (properties, createdAt, etc.)
        const fullNode = props.nodes.find(n => n.id === nodeData.id) ?? null;
        onNodeSelect(fullNode);
      }
    });

    // Handle background tap — intentionally does nothing to preserve selection

    // Update label visibility based on zoom level
    const updateLabelVisibility = () => {
      if (!cy) return;
      const zoom = cy.zoom();
      
      // Remove all labels first
      cy.nodes().removeClass('show-label');
      
      if (zoom >= 2.5) {
        // Very zoomed in: show all labels
        cy.nodes().addClass('show-label');
      } else if (zoom >= 1.8) {
        // Medium zoom: show selected + connected
        cy.nodes(':selected').addClass('show-label');
        cy.nodes('.connected').addClass('show-label');
      } else {
        // Default: only show selected node label
        cy.nodes(':selected').addClass('show-label');
      }
      
      // Update edge labels based on zoom
      if (zoom >= 2.0) {
        // Show edge labels when zoomed in enough
        cy.edges('.relevant').style('label', 'data(relation)');
      } else {
        // Hide edge labels at lower zoom
        cy.edges('.relevant').style('label', '');
      }
    };

    cy.on('zoom', updateLabelVisibility);
    cy.on('pan', updateLabelVisibility);

    // Load initial data
    updateGraphData();

    // Expose a test helper to trigger a node tap directly from tests. This is
    // enabled only when the test harness sets `globalThis.__TEST__ = true` so
    // it does not affect production behavior.
    if ((globalThis as { __TEST__?: boolean }).__TEST__) {
      (globalThis as { __triggerGraphTap?: (id: string) => void }).__triggerGraphTap = (id: string) => {
        const triggerArg = { target: { data: () => ({ id }) } } as unknown as object;
        // pass a plain object as the event-like argument; cast via unknown->never to avoid `any`
        cy?.trigger('tap', triggerArg as unknown as never);
      };
    }
  });

  // Helper to update graph data
  const updateGraphData = () => {
    if (!cy) return;

    const elements = [
      ...props.nodes.map(node => ({
        data: { 
          id: node.id, 
          label: node.label ?? '', 
          type: node.type ?? '',
          value: node.value ?? '',
        }
      })),
      ...props.edges.map(edge => ({
        data: {
          id: `${edge.sourceId}-${edge.targetId}`,
          source: edge.sourceId,
          target: edge.targetId,
          relation: edge.relation ?? '',
        }
      }))
    ];

    cy.elements().remove();
    cy.add(elements);
    
    const layout = cy.layout({ 
      name: 'cose',
      animate: false,
      nodeRepulsion: 4000,
      nodeOverlap: 20,
      idealEdgeLength: 40,
      edgeElasticity: 80,
      nestingFactor: 1.2,
      gravity: 1.5,
      numIter: 1000,
      initialTemp: 200,
      coolingFactor: 0.95,
      minTemp: 1.0,
    } as Parameters<typeof cy.layout>[0]);
    
    layout.run();
    
    // Fit to view after layout completes
    setTimeout(() => {
      cy?.fit(undefined, 50);
    }, 100);
  };

  // Update graph data ONLY when node/edge count changes (prevent unnecessary re-layouts)
  createEffect(() => {
    const nodeCount = props.nodes.length;
    const edgeCount = props.edges.length;
    
    if (nodeCount !== lastNodeCount || edgeCount !== lastEdgeCount) {
      lastNodeCount = nodeCount;
      lastEdgeCount = edgeCount;
      updateGraphData();
    }
  });

  // Update selection highlighting
  createEffect(() => {
    if (!cy) return;

    const selectedId = props.selectedNodeId;
    
    // Always reset all state first
    cy.nodes().unselect();
    cy.nodes().removeClass('connected show-label');
    cy.edges().removeClass('relevant dimmed');

    if (selectedId) {
      const selectedNode = cy.getElementById(selectedId);
      
      if (selectedNode.length > 0) {
        // Highlight connected nodes
        const connectedNodes = selectedNode.neighborhood('node');
        connectedNodes.addClass('connected');
        
        // Highlight relevant edges
        const connectedEdges = selectedNode.connectedEdges();
        connectedEdges.addClass('relevant');
        
        // Dim other edges
        cy.edges().not(connectedEdges).addClass('dimmed');
        
        // Select the node and show its label
        selectedNode.select();
        selectedNode.addClass('show-label');
      }
    }
  });

  onCleanup(() => {
    cy?.destroy();
  });

  return (
    <div class="graph-visualization">
      <div ref={containerRef} class="graph-container" />
      <Show when={!cytoscapeLoaded()}>
        <div class="graph-loading">Loading graph...</div>
      </Show>
      <Show when={cytoscapeLoaded()}>
        <div class="graph-controls">
        <button 
          class="graph-btn" 
          onClick={() => cy?.fit(undefined, 50)}
          title="Fit to view"
        >
          <span class="material-symbols-outlined">fit_screen</span>
        </button>
        <button 
          class="graph-btn" 
          onClick={() => cy?.zoom(cy.zoom() * 1.2)}
          title="Zoom in"
        >
          <span class="material-symbols-outlined">add</span>
        </button>
        <button 
          class="graph-btn" 
          onClick={() => cy?.zoom(cy.zoom() * 0.8)}
          title="Zoom out"
        >
          <span class="material-symbols-outlined">remove</span>
        </button>
        <button 
          class="graph-btn" 
          onClick={() => cy?.reset()}
          title="Reset view"
        >
          <span class="material-symbols-outlined">refresh</span>
        </button>
      </div>
      </Show>
    </div>
  );
}
