// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { describe, it, expect } from 'vitest';
import {
  UPDATE_CONFIG_MUTATION,
  ADD_MCP_SERVER_MUTATION,
  REMOVE_MCP_SERVER_MUTATION,
  ADD_TASK_MUTATION,
  REMOVE_TASK_MUTATION,
} from './mutations';

/**
 * These tests verify that each exported mutation string exists, is a
 * non-empty string, contains the expected GraphQL operation name, and
 * declares the correct top-level field so that the backend schema can
 * route the request correctly.
 */
describe('GraphQL mutations', () => {
  // ------------------------------------------------------------------ //
  // UPDATE_CONFIG_MUTATION                                               //
  // ------------------------------------------------------------------ //

  it('UPDATE_CONFIG_MUTATION is a non-empty string', () => {
    expect(typeof UPDATE_CONFIG_MUTATION).toBe('string');
    expect(UPDATE_CONFIG_MUTATION.trim().length).toBeGreaterThan(0);
  });

  it('UPDATE_CONFIG_MUTATION contains mutation keyword', () => {
    expect(UPDATE_CONFIG_MUTATION).toContain('mutation');
  });

  it('UPDATE_CONFIG_MUTATION contains UpdateConfig operation', () => {
    expect(UPDATE_CONFIG_MUTATION).toContain('UpdateConfig');
  });

  it('UPDATE_CONFIG_MUTATION references $input variable', () => {
    expect(UPDATE_CONFIG_MUTATION).toContain('$input');
  });

  it('UPDATE_CONFIG_MUTATION selects agentName field', () => {
    expect(UPDATE_CONFIG_MUTATION).toContain('agentName');
  });

  it('UPDATE_CONFIG_MUTATION selects channels field', () => {
    expect(UPDATE_CONFIG_MUTATION).toContain('channels');
  });

  // ------------------------------------------------------------------ //
  // ADD_MCP_SERVER_MUTATION                                             //
  // ------------------------------------------------------------------ //

  it('ADD_MCP_SERVER_MUTATION is a non-empty string', () => {
    expect(typeof ADD_MCP_SERVER_MUTATION).toBe('string');
    expect(ADD_MCP_SERVER_MUTATION.trim().length).toBeGreaterThan(0);
  });

  it('ADD_MCP_SERVER_MUTATION contains mutation keyword', () => {
    expect(ADD_MCP_SERVER_MUTATION).toContain('mutation');
  });

  it('ADD_MCP_SERVER_MUTATION contains AddMCPServer operation', () => {
    expect(ADD_MCP_SERVER_MUTATION).toContain('AddMCPServer');
  });

  it('ADD_MCP_SERVER_MUTATION references $name, $transport, $command variables', () => {
    expect(ADD_MCP_SERVER_MUTATION).toContain('$name');
    expect(ADD_MCP_SERVER_MUTATION).toContain('$transport');
    expect(ADD_MCP_SERVER_MUTATION).toContain('$command');
  });

  it('ADD_MCP_SERVER_MUTATION selects id and status fields', () => {
    expect(ADD_MCP_SERVER_MUTATION).toContain('id');
    expect(ADD_MCP_SERVER_MUTATION).toContain('status');
  });

  // ------------------------------------------------------------------ //
  // REMOVE_MCP_SERVER_MUTATION                                          //
  // ------------------------------------------------------------------ //

  it('REMOVE_MCP_SERVER_MUTATION is a non-empty string', () => {
    expect(typeof REMOVE_MCP_SERVER_MUTATION).toBe('string');
    expect(REMOVE_MCP_SERVER_MUTATION.trim().length).toBeGreaterThan(0);
  });

  it('REMOVE_MCP_SERVER_MUTATION contains RemoveMCPServer operation', () => {
    expect(REMOVE_MCP_SERVER_MUTATION).toContain('RemoveMCPServer');
  });

  it('REMOVE_MCP_SERVER_MUTATION references $id variable', () => {
    expect(REMOVE_MCP_SERVER_MUTATION).toContain('$id');
  });

  it('REMOVE_MCP_SERVER_MUTATION selects success and error fields', () => {
    expect(REMOVE_MCP_SERVER_MUTATION).toContain('success');
    expect(REMOVE_MCP_SERVER_MUTATION).toContain('error');
  });

  // ------------------------------------------------------------------ //
  // ADD_TASK_MUTATION                                                    //
  // ------------------------------------------------------------------ //

  it('ADD_TASK_MUTATION is a non-empty string', () => {
    expect(typeof ADD_TASK_MUTATION).toBe('string');
    expect(ADD_TASK_MUTATION.trim().length).toBeGreaterThan(0);
  });

  it('ADD_TASK_MUTATION contains AddTask operation', () => {
    expect(ADD_TASK_MUTATION).toContain('AddTask');
  });

  it('ADD_TASK_MUTATION references required task variables', () => {
    expect(ADD_TASK_MUTATION).toContain('$name');
    expect(ADD_TASK_MUTATION).toContain('$prompt');
    expect(ADD_TASK_MUTATION).toContain('$schedule');
    expect(ADD_TASK_MUTATION).toContain('$channel');
    expect(ADD_TASK_MUTATION).toContain('$isCyclic');
  });

  it('ADD_TASK_MUTATION selects id and isCyclic fields', () => {
    expect(ADD_TASK_MUTATION).toContain('id');
    expect(ADD_TASK_MUTATION).toContain('isCyclic');
  });

  // ------------------------------------------------------------------ //
  // REMOVE_TASK_MUTATION                                                 //
  // ------------------------------------------------------------------ //

  it('REMOVE_TASK_MUTATION is a non-empty string', () => {
    expect(typeof REMOVE_TASK_MUTATION).toBe('string');
    expect(REMOVE_TASK_MUTATION.trim().length).toBeGreaterThan(0);
  });

  it('REMOVE_TASK_MUTATION contains RemoveTask operation', () => {
    expect(REMOVE_TASK_MUTATION).toContain('RemoveTask');
  });

  it('REMOVE_TASK_MUTATION references $taskId variable', () => {
    expect(REMOVE_TASK_MUTATION).toContain('$taskId');
  });

  it('REMOVE_TASK_MUTATION selects success and error fields', () => {
    expect(REMOVE_TASK_MUTATION).toContain('success');
    expect(REMOVE_TASK_MUTATION).toContain('error');
  });
});
