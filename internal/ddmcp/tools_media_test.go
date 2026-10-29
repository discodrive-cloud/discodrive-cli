package ddmcp

import "testing"

func TestMediaDefaultHidesChanging(t *testing.T) {
	all := append(append(toolSpecs(), pimToolSpecs()...), mediaToolSpecs()...)
	got := names(selectTools(all, false))
	for _, want := range []string{"dd_music_list", "dd_music_radio_list", "dd_music_podcasts_list", "dd_books_list", "dd_books_facets", "dd_books_meta"} {
		if !got[want] {
			t.Fatalf("read tool %q must be available by default", want)
		}
	}
	for _, banned := range []string{"dd_music_radio_delete", "dd_music_podcast_delete", "dd_books_meta_update", "dd_music_radio_add", "dd_music_podcast_add"} {
		if got[banned] {
			t.Fatalf("mutating or URL-fetching tool %q must not be available without --allow-changes", banned)
		}
	}
}

func TestMediaAllowChangesExposesAll(t *testing.T) {
	all := append(append(toolSpecs(), pimToolSpecs()...), mediaToolSpecs()...)
	got := names(selectTools(all, true))
	for _, want := range []string{"dd_music_radio_delete", "dd_music_podcast_delete", "dd_books_meta_update"} {
		if !got[want] {
			t.Fatalf("mutating tool %q must be available under --allow-changes", want)
		}
	}
}
