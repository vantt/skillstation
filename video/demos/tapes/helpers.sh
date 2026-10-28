# shellcheck shell=bash
# Sourced off camera by the tapes, after sandbox.sh. Off-camera steps that the video shows another way.

# Step 2 of "Create your first skill", from the same fixtures and in the same way as happy-path.sh:
# replace the scaffold placeholder under Instructions of SKILL.md and add two requests under `examples` in skill.yaml.
video_write_instructions() {
	FIX_INSTRUCTIONS=$DEMO_DIR/fixtures/commit-style/instructions.md \
		perl -0pi -e '
		sub slurp { open my $fh, "<", $_[0] or die "$_[0]: $!\n"; local $/; my $t = <$fh>; return $t }
		my $instr = slurp($ENV{FIX_INSTRUCTIONS});
		s/<!-- skillstation:scaffold -->\n(?:[^\n]+\n?)*/$instr/ or die "no scaffold placeholder in SKILL.md\n";
	' .agents/skills/commit-style/SKILL.md &&
		FIX_EXAMPLES=$DEMO_DIR/fixtures/commit-style/more-examples.md perl -0pi -e '
		sub slurp { open my $fh, "<", $_[0] or die "$_[0]: $!\n"; local $/; my $t = <$fh>; return $t }
		my $more = slurp($ENV{FIX_EXAMPLES});
		s/(    examples:\n(?:        - [^\n]*\n)+)/$1$more/ or die "no examples list in skill.yaml\n";
	' .agents/skills/commit-style/skill.yaml
}
