// Utilities to pull the HLS stream

package main

// Pulls HLS stream from HLS source
func (ch *ConnectionHandler) PullFromHlsSource(source *HlsSource, pullingInterruptChannel chan bool, maxInitialFragments int) {
	listenSuccess, listenChan, initialFragments := source.AddListener(ch.id)

	if !listenSuccess {
		ch.SendClose()
		return
	}

	defer source.RemoveListener(ch.id)

	ch.PullStream(listenChan, pullingInterruptChannel, initialFragments, maxInitialFragments)
}

// Pull stream from events channel and initial fragments list
func (ch *ConnectionHandler) PullStream(listenChan chan HlsEvent, pullingInterruptChannel chan bool, initialFragments []*HlsFragment, maxInitialFragments int) {
	// Send initial fragments

	if maxInitialFragments < 0 {
		maxInitialFragments = len(initialFragments)
	}

	for i := 0; i < len(initialFragments) && i < maxInitialFragments; i++ {
		ch.SendFragment(initialFragments[i])
	}

	// Listen for events

	for {
		select {
		case ev := <-listenChan:
			if ev.EventType == HLS_EVENT_TYPE_CLOSE {
				ch.SendClose()
				return
			}

			if ev.Fragment != nil {
				ch.SendFragment(ev.Fragment)
			}
		case <-pullingInterruptChannel:
			return
		}
	}
}
