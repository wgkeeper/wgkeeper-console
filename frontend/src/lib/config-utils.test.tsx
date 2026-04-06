import { describe, it, expect } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { renderConfig } from './config-utils';

const render = (text: string) => renderToStaticMarkup(<>{renderConfig(text)}</>);

describe('renderConfig', () => {
  describe('section headers', () => {
    it('renders [Interface] as font-semibold', () => {
      const html = render('[Interface]');
      expect(html).toContain('font-semibold');
      expect(html).toContain('[Interface]');
    });

    it('renders [Peer] as font-semibold', () => {
      const html = render('[Peer]');
      expect(html).toContain('font-semibold');
    });

    it('requires closing bracket — unclosed bracket is not a section', () => {
      const html = render('[Interface');
      expect(html).not.toContain('font-semibold');
    });

    it('trims whitespace before checking section syntax', () => {
      const html = render('  [Interface]  ');
      expect(html).toContain('font-semibold');
    });
  });

  describe('comments', () => {
    it('renders # comment as italic muted', () => {
      const html = render('# this is a comment');
      expect(html).toContain('italic');
      expect(html).toContain('# this is a comment');
    });

    it('renders ; comment as italic muted', () => {
      const html = render('; semicolon comment');
      expect(html).toContain('italic');
      expect(html).toContain('; semicolon comment');
    });

    it('trims whitespace before checking comment prefix', () => {
      const html = render('  # indented comment');
      expect(html).toContain('italic');
    });
  });

  describe('key = value lines', () => {
    it('splits on = and renders key and value separately', () => {
      const html = render('PrivateKey = abc123');
      expect(html).toContain('PrivateKey ');
      expect(html).toContain(' abc123');
    });

    it('splits only on the first = (value may contain =)', () => {
      const html = render('PreSharedKey = a=b=c');
      expect(html).toContain('PreSharedKey ');
      expect(html).toContain(' a=b=c');
    });

    it('renders the = separator', () => {
      const html = render('DNS = 1.1.1.1');
      expect(html).toContain('=');
    });
  });

  describe('plain lines', () => {
    it('renders unclassified lines as plain text', () => {
      const html = render('plaintext');
      expect(html).toContain('plaintext');
    });

    it('does not apply semibold or italic to plain lines', () => {
      const html = render('plaintext');
      expect(html).not.toContain('font-semibold');
      expect(html).not.toContain('italic');
    });
  });

  describe('multi-line config', () => {
    it('returns one element per line', () => {
      expect(renderConfig('[Interface]\nPrivateKey = abc\nDNS = 1.1.1.1')).toHaveLength(3);
    });

    it('inserts \\n between lines but not after the last', () => {
      const result = renderConfig('a\nb\nc');
      const html = renderToStaticMarkup(<>{result}</>);
      // two newlines for three lines: between 1-2 and 2-3
      expect(html.split('\n')).toHaveLength(3);
    });

    it('handles empty string as single empty line', () => {
      expect(renderConfig('')).toHaveLength(1);
    });

    it('renders a full WireGuard config block without throwing', () => {
      const config = [
        '[Interface]',
        'PrivateKey = AAAA',
        'Address = 10.0.0.2/32',
        'DNS = 1.1.1.1',
        '',
        '[Peer]',
        '# server peer',
        'PublicKey = BBBB',
        'AllowedIPs = 0.0.0.0/0',
        'Endpoint = example.com:51820',
      ].join('\n');
      expect(() => render(config)).not.toThrow();
      expect(render(config)).toContain('font-semibold');
      expect(render(config)).toContain('italic');
    });
  });
});
