export function Footer() {
  return (
    <footer className="border-t bg-background mt-auto py-4">
      <div className="container mx-auto px-4 flex items-center justify-center gap-4 text-sm text-muted-foreground">
        <span>v1.0.1</span>
        <span className="text-muted-foreground/50">|</span>
        <a
          href="https://github.com/Lampon/PanCheck"
          target="_blank"
          rel="noopener noreferrer"
          className="hover:text-foreground transition-colors underline-offset-4 hover:underline"
        >
          GitHub
        </a>
      </div>
    </footer>
  );
}

