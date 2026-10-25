class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "3"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v3/grubby_v3_darwin_arm64.tar.gz"
      sha256 "923d9d63a024511e00262697c99c903ff1459062bf44341308ad63bb6a7b36af"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v3/grubby_v3_darwin_amd64.tar.gz"
      sha256 "6eb203d2cf10581da3421466ba81d1dbac01729dc365e0fa108b3560375af54e"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v3/grubby_v3_linux_arm64.tar.gz"
      sha256 "6b9623ca322149ba38ad8b07c55b88a229f1d148453b6bd5b489925dd71d6c8b"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v3/grubby_v3_linux_amd64.tar.gz"
      sha256 "f7c95b89a280a567b3f968df63aebbe157a84c9008f976a7ca7719a7e682ce00"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
