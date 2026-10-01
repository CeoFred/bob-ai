import React, { useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Copy, Check } from 'lucide-react';

interface MarkdownRendererProps {
  content?: string;
  className?: string;
}

interface CodeBlockProps {
  inline?: boolean;
  className?: string;
  children?: React.ReactNode;
}

const CodeBlock: React.FC<CodeBlockProps> = ({ inline, className, children }) => {
  const [copied, setCopied] = useState(false);
  const match = /language-(\w+)/.exec(className || '');
  const language = match ? match[1] : '';
  const codeString = String(children).replace(/\n$/, '');

  // If it's inline code (single line or specifically inline)
  if (inline || (!match && !codeString.includes('\n'))) {
    return (
      <code className="px-1.5 py-0.5 mx-0.5 rounded-md bg-zinc-850 border border-zinc-750/70 text-amber-200/95 font-mono text-[11px] sm:text-xs tracking-tight break-all">
        {children}
      </code>
    );
  }

  const handleCopy = () => {
    navigator.clipboard.writeText(codeString);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <div className="my-3 rounded-xl border border-zinc-800/80 bg-zinc-950 overflow-hidden shadow-md">
      <div className="flex items-center justify-between px-3 py-1.5 bg-zinc-900/90 border-b border-zinc-800 text-[11px] font-mono text-zinc-400 select-none">
        <span className="font-semibold text-zinc-300 uppercase tracking-wider text-[10px]">
          {language || 'code'}
        </span>
        <button
          onClick={handleCopy}
          className="flex items-center gap-1.5 text-zinc-400 hover:text-zinc-200 transition-colors cursor-pointer text-[10px] px-2 py-0.5 rounded bg-zinc-800/60 hover:bg-zinc-800 border border-zinc-700/50"
          title="Copy code"
        >
          {copied ? (
            <>
              <Check className="w-3 h-3 text-emerald-400" />
              <span className="text-emerald-400">Copied</span>
            </>
          ) : (
            <>
              <Copy className="w-3 h-3" />
              <span>Copy</span>
            </>
          )}
        </button>
      </div>
      <pre className="p-3.5 overflow-x-auto text-xs sm:text-sm font-mono leading-relaxed text-zinc-200">
        <code>{children}</code>
      </pre>
    </div>
  );
};

export const MarkdownRenderer: React.FC<MarkdownRendererProps> = ({ content = '', className = '' }) => {
  return (
    <div className={`markdown-content space-y-2 text-zinc-200 leading-relaxed ${className}`}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          code({ inline, className: codeClassName, children }: any) {
            return (
              <CodeBlock inline={inline} className={codeClassName}>
                {children}
              </CodeBlock>
            );
          },
          h1: ({ children }) => (
            <h1 className="text-base sm:text-lg font-bold text-zinc-100 mt-4 mb-2 pb-1 border-b border-zinc-800/80 first:mt-0">
              {children}
            </h1>
          ),
          h2: ({ children }) => (
            <h2 className="text-sm sm:text-base font-semibold text-zinc-100 mt-3.5 mb-1.5 pb-0.5 border-b border-zinc-800/50 first:mt-0">
              {children}
            </h2>
          ),
          h3: ({ children }) => (
            <h3 className="text-xs sm:text-sm font-semibold text-zinc-100 mt-3 mb-1 first:mt-0">
              {children}
            </h3>
          ),
          h4: ({ children }) => (
            <h4 className="text-xs sm:text-xs font-semibold text-zinc-200 mt-2.5 mb-1 first:mt-0">
              {children}
            </h4>
          ),
          p: ({ children }) => <p className="mb-2 last:mb-0 leading-relaxed text-zinc-200">{children}</p>,
          ul: ({ children }) => (
            <ul className="list-disc list-outside ml-4 sm:ml-5 space-y-1 my-2 text-zinc-200 marker:text-zinc-500">
              {children}
            </ul>
          ),
          ol: ({ children }) => (
            <ol className="list-decimal list-outside ml-4 sm:ml-5 space-y-1 my-2 text-zinc-200 marker:text-zinc-500 font-normal">
              {children}
            </ol>
          ),
          li: ({ children }) => <li className="leading-relaxed pl-1 text-zinc-200">{children}</li>,
          blockquote: ({ children }) => (
            <blockquote className="border-l-2 border-zinc-600 bg-zinc-900/40 pl-3.5 py-1.5 my-2.5 rounded-r-lg text-zinc-300 italic">
              {children}
            </blockquote>
          ),
          table: ({ children }) => (
            <div className="overflow-x-auto rounded-lg border border-zinc-800 my-3 bg-zinc-950/40 shadow-sm">
              <table className="w-full text-left border-collapse text-xs text-zinc-200">{children}</table>
            </div>
          ),
          thead: ({ children }) => <thead className="bg-zinc-900/90 border-b border-zinc-800">{children}</thead>,
          tbody: ({ children }) => <tbody className="divide-y divide-zinc-800/60">{children}</tbody>,
          tr: ({ children }) => <tr className="hover:bg-zinc-900/30 transition-colors">{children}</tr>,
          th: ({ children }) => (
            <th className="px-3 py-2 font-semibold text-zinc-300 text-xs tracking-wide">{children}</th>
          ),
          td: ({ children }) => <td className="px-3 py-2 text-xs text-zinc-300 leading-normal">{children}</td>,
          a: ({ href, children }) => (
            <a
              href={href}
              target="_blank"
              rel="noopener noreferrer"
              className="text-blue-400 hover:text-blue-300 underline underline-offset-2 transition-colors font-medium"
            >
              {children}
            </a>
          ),
          strong: ({ children }) => <strong className="font-semibold text-zinc-100">{children}</strong>,
          em: ({ children }) => <em className="italic text-zinc-300">{children}</em>,
          hr: () => <hr className="my-3.5 border-zinc-800" />,
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
};
