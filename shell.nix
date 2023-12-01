{}:

let
	pkgs = import ./nix/pkgs.nix {};
in

with pkgs.lib;
with builtins;

pkgs.mkShell {
	buildInputs = with pkgs; [
		bash
		niv
		jq
		moreutils # for parallel
		ffmpeg-full

		# Go tools.
		go
		gopls
		gotools
		go-tools # staticcheck
	];

	shellHook = ''
		export PATH="$PATH:${toString ./.}/bin"
	'';
}
