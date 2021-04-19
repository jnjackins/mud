package main

var (
	stopWords = []string{
		"myself", "ours", "ourselves", "your", "yours", "yourself", "yourselves", "himself",
		"herself", "itself", "they", "them", "their", "theirs", "themselves", "what", "which", "whom", "this", "that",
		"these", "those", "were", "been", "being", "have", "having", "does", "doing", "because", "until", "while",
		"with", "about", "against", "between", "into", "through", "during", "before", "after", "above", "below",
		"from", "down", "over", "under", "again", "further", "then", "once", "here", "there", "when", "where",
		"both", "each", "more", "most", "other", "some", "such", "only", "same", "than", "very", "will",
		"just", "should", "stands", "flies", "tries", "north", "south", "east", "west", "atop", "towards", "exits", "walks",
		"generally",
	}
	stopWordsMap map[string]struct{}
)

func init() {
	stopWordsMap = make(map[string]struct{}, len(stopWords))
	for _, word := range stopWords {
		stopWordsMap[word] = struct{}{}
	}
}

func interesting(word string) bool {
	if len(word) < 4 {
		return false
	}
	_, member := stopWordsMap[word]
	return !member
}
