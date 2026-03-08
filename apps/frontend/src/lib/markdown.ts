/**
 * Markdown renderer for LLM output.
 * Uses markdown-it (CommonMark + linkify, typographer).
 * Bundled in vendor-markdown chunk.
 */
import MarkdownIt from "markdown-it";

const md = new MarkdownIt({
  html: false, // Disable raw HTML for safety (LLM output)
  linkify: true,
  typographer: true,
});

export function renderMarkdown(content: string): string {
  const processed = content.replace(/\\n/g, "\n");
  return md.render(processed);
}
